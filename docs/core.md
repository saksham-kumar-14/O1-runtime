# Core architecture

## Isolation (Namespaces and Chroot)
- **Namespaces:** Utilizes `CLONE_NEWPID`, `CLONE_NEWUTS`, `CLONE_NEWNS`, and `CLONE_NEWNET` to create a sterile execution environment. The container cannot see host processes, host networks, or host mount points.
- **filesystem:** Implements `syscall.PivotRoot` to physically teleport the process into the new root filesystem, ensuring it cannot traverse back up to the host OS.

## Storage
- **Layered Filesystem:** Uses Linux `OverlayFS` to stack a temporary, writable `upperdir` and `workdir` on top of a read only Linux base image (`lowerdir`).
- **Virtual Devices:** Automatically mounts devtmpfs into the container's `/dev` directory to provide standard Unix I/O devices (like `/dev/null`), enabling background process execution (`&`).
- **Persistent Volumes:** Supports host-to-container directory mapping using `MS_BIND | MS_REC syscalls` for database and source code persistence.

## Cgroups
- Dynamically generates independent control groups (`/sys/fs/cgroup/o1-runtime/<PID>`) for every container.
- Memory Limit: 512 MB
- Process Limit: 20 maximum background/foreground processes per container.

## 4. Networking
By default, the isolated network namespace has no connection to the outside world. So, build a custom virtual network from scratch.
- **The Virtual Switch (`o1-br0`):** A fake network bridge on the host and assign it the IP `10.0.0.1`. Every container uses this as its default gateway (its router).
- **Enabling Routing:** Run `sysctl -w net.ipv4.ip_forward=1` to tell the kernel it is allowed to act as a router, and add `iptables` rules to allow traffic in and out of the bridge.
- **Virtual Cables (Veth Pairs):** A virtual ethernet cable. One end stays on the host and plugs into `o1-br0`. Used `nsenter` to push the other end through the namespace boundary directly into the container.
- **Container Network Config:** Once the cable is inside, assign the container a dynamic IP (e.g., `10.0.0.2`), turn on the `localhost` loopback interface, and set its default route to send all traffic out to `10.0.0.1`.
- **Outbound Internet (MASQUERADE):** Because the container uses a fake internal IP (`10.0.0.x`), the real internet will drop its packets. Used a Source NAT (`MASQUERADE`) `iptables` rule. When a packet leaves the host, the kernel temporarily swaps the container's fake IP for the host's real IP.
- **Inbound Port Forwarding:** To host a web server, use `DNAT` rules. If external traffic hits the host on port `9000`, `iptables` intercepts it and forwards it down the virtual cable to the container's internal port `80`.
- **DNS Resolution:** Right before boot, the engine creates an `/etc/resolv.conf` file inside the container pointing to public nameservers (like `8.8.8.8`) so the container can translate domain names like `google.com`.


## State Management
`o1` utilizes a daemonless, file-system-driven architecture to track and manage container states. The control plane relies on Linux signals and namespace injection to manage running isolated processes.

- **State Persistence:** No background service to track container. Instead upon creation, container's configuration is serialized into a JSON file stored at `/var/lib/o1/state/`.
- **Process Probing (Signal 0):** To determine container health without a daemon, the engine utilizes the Unix `Signal 0` syscall (`syscall.Kill(PID, 0)`). If kernel throws error while pining the PID, it is marked as *dead*.
- **Namespace Injection:** The engine uses namespace breaching using `nsenter`. The engine queries the state database for the container's Host PID.
  - It executes `nsenter` targeting the Mount (`-m`), UTS (`-u`), IPC (`-i`), Network (`-n`), and PID (`-p`) namespaces of that target PID.
  - Host standard streams (`stdin`, `stdout`, `stderr`) are wired directly into the injected process, creating a seamless interactive shell session.
- **Stop:** Sends a graceful `SIGTERM` signal to the process, allowing the application to safely shut down. It clears the state database and OverlayFS workspace but leaves network routing intact.
- **Remove** Executes a complete system cleanse. It sends an uncatchable `SIGKILL` to the process, deletes the virtual ethernet (`veth`) interface from the host bridge, wipes the OverlayFS directories, and completely flushes the `iptables` NAT table to prevent ghost routing rules.
