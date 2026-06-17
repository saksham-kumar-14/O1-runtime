# O1 runtime
`o1` is a lightweight, daemonless Linux container engine built in Go.

## Features
- **Daemonless Architecture:** No background processes. State is driven by filesystem and kernel syscalls.
- **Docker Hub Integration:** Support for pulling multi-architecture OCI images from Docker Hub.
- **Image Builder:** Built-in `Dockerfile` parser and diff-harvesting engine to build custom container images directly from source (`o1 build`).
- **Zero-Config OCI Execution:** Automatically parses image manifests (`config.json`) to natively resolve `Entrypoint`, `Cmd`, and default `Env` variables.
- **Kernel Security (Seccomp BPF):** Injects a compiled system call firewall directly into the Linux kernel to block malicious container escapes (restricting `mount`, `ptrace`, `unshare`, etc.).
- **Overlay Filesystem:** Dynamic layered, copy-on-write filesystem for instant provisioning.
- **Network Namespace Bridging:** Isolated container networking with custom IPAM, host-to-container communication, and NAT masquerading for internet access.
- **Cgroups v2 Resource Management:** Dynamic limits for CPU, memory, and PIDs (`--cpus`, `--memory`, `--cpuset`, `--pids`).
- **Port Forwarding:** Dynamic `iptables` routing for single or multiple ports.
- **Volumes / Bind Mounts:** Persist data by mapping host directories directly into the container.
- **Environment Layering:** Securely scrubs the host environment and intelligently merges image-defined variables with user-injected overrides.
- **Global Privilege Enforcement:** Native root-level execution verification to prevent cryptic namespace and networking failures.

## CLI Commands
- `o1 run [args] <image> [command]`: Spawn a new isolated container (command falls back to OCI manifest defaults if omitted)
- `o1 build <file> <name>`: Parse a Dockerfile, execute build steps in an ephemeral container, and harvest a new custom image layer
- `o1 pull <image>`: Download an image from Docker Hub
- `o1 ps`: List all actively running containers
- `o1 stats`: Live dashboard of CPU & RAM utilization
- `o1 exec <id> <command>`: Inject a new process into a running container's namespace
- `o1 logs <id>`: Fetch standard output/error logs from a container
- `o1 stop <id>`: Send a graceful `SIGTERM` to a running container
- `o1 rm <id>`: Send `SIGKILL` and forcefully remove network/cgroup resources
- `o1 images`: List locally downloaded images
- `o1 rmi <image>`: Delete a local image

> Read the architecture at [architecture docs](./docs/core.md)
