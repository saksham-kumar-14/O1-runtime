# Core Architecture

## Isolation (Namespaces and Chroot)
- **Namespaces:** Utilizes `CLONE_NEWPID`, `CLONE_NEWUTS`, `CLONE_NEWNS`, and `CLONE_NEWNET` to create a execution environment. The container cannot see host processes, host networks, or host mount points.
- **Filesystem:** Implements `syscall.PivotRoot` to physically teleport the process into the new root filesystem, ensuring it cannot traverse back up to the host OS.

## Storage
- **OCI Image Registry:** Natively communicates with the Docker Hub v2 API to authenticate, resolve Multi-Architecture manifests (`amd64`/`arm64`), and download compressed filesystem layers.
- **Layered Filesystem:** Uses Linux `OverlayFS` to stack a temporary, writable `upperdir` and `workdir` on top of the extracted, read-only Docker image layers (`lowerdir`).
- **Virtual Devices:** Automatically mounts `devtmpfs` into the container's `/dev` directory to provide standard Unix I/O devices (like `/dev/null` and `/dev/urandom`), enabling background process execution.
- **Persistent Volumes:** Supports host-to-container directory mapping using `MS_BIND | MS_REC` syscalls for database and source code persistence.

## Resource Management (Cgroups v2)
- Dynamically generates independent control groups (`/sys/fs/cgroup/o1-runtime/<ID>`) for every container using the Linux Cgroups v2 unified hierarchy.
- **Dynamic Allocation:** Limits are no longer hardcoded. The engine parses CLI flags to dynamically throttle hardware access via kernel files:
  - `--memory`: Writes to `memory.max` (e.g., RAM limits).
  - `--cpus`: Calculates microseconds and writes to `cpu.max` for strict CPU throttling.
  - `--cpuset`: Writes to `cpuset.cpus` for physical CPU core pinning.
  - `--pids`: Writes to `pids.max` to prevent fork-bomb attacks.

## Networking
By default, the isolated network namespace has no connection to the outside world. `o1` builds a custom virtual network from scratch:
- **The Virtual Switch (`o1-br0`):** A fake network bridge on the host assigned the IP `10.0.0.1`. Every container uses this as its default gateway (router).
- **Enabling Routing:** Runs `sysctl -w net.ipv4.ip_forward=1` to allow the kernel to act as a router, and adds `iptables` rules to allow traffic in and out of the bridge.
- **Virtual Cables (Veth Pairs):** A virtual ethernet cable. One end stays on the host and plugs into `o1-br0`. Uses `nsenter` to push the other end through the namespace boundary directly into the container.
- **Container Network Config:** Once the cable is inside, the engine assigns the container a dynamic IP (e.g., `10.0.0.2`), turns on the `localhost` loopback interface, and sets its default route to send all traffic out to `10.0.0.1`.
- **Outbound Internet (MASQUERADE):** Because the container uses a fake internal IP (`10.0.0.x`), the real internet will drop its packets. Uses a Source NAT (`MASQUERADE`) `iptables` rule. When a packet leaves the host, the kernel temporarily swaps the container's fake IP for the host's real IP.
- **Inbound Port Forwarding:** To host a web server, the engine uses `DNAT` rules. If external traffic hits the host on a mapped port (e.g., `8080`), `iptables` intercepts it and forwards it down the virtual cable to the container's internal port (e.g., `80`).
- **DNS Resolution:** Right before boot, the engine injects an `/etc/resolv.conf` file inside the container pointing to public nameservers (like `8.8.8.8`) so the container can translate domain names.

## State Management
`o1` utilizes a daemonless, file-system-driven architecture to track and manage container states. The control plane relies on Linux signals and namespace injection rather than a background API service.
- **State Persistence:** Upon creation, the container's configuration is serialized into a JSON file stored at `/var/lib/o1/state/`.
- **Process Probing (Signal 0):** To determine container health without a daemon, the engine utilizes the Unix `Signal 0` syscall (`syscall.Kill(PID, 0)`). If the kernel throws an error while pinging the PID, the container is marked as *Dead*.
- **Namespace Injection:** To execute commands in running containers, the engine queries the state database for the container's Host PID, then executes `nsenter` targeting the Mount (`-m`), UTS (`-u`), IPC (`-i`), Network (`-n`), and PID (`-p`) namespaces. Host standard streams (`stdin`, `stdout`, `stderr`) are wired directly into the injected process, creating a seamless interactive shell.
- **Stop:** Sends a graceful `SIGTERM` signal to the process, allowing the application to safely shut down. It clears the state database and OverlayFS workspace but leaves network routing intact.
- **Remove:** Executes a complete system cleanse. It sends an uncatchable `SIGKILL` to the process, deletes the virtual ethernet (`veth`) interface from the host bridge, wipes the OverlayFS directories, and completely flushes the container's specific `iptables` NAT rules to prevent ghost routing.
- **Stats:** Provides live, daemonless telemetry by querying the Linux Kernel's Cgroup v2 virtual filesystem directly. It continuously reads `memory.current` for real-time RAM consumption and `cpu.stat` for total CPU execution time (`usage_usec`). CPU utilization percentage is calculated dynamically by measuring the time-delta of microsecond execution against a 1-second refresh interval. The dashboard is rendered in-place using ANSI terminal escape sequences.
