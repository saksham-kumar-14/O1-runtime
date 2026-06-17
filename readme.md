# O1 runtime
`o1` is a lightweight, daemonless Linux container engine built in Go.

## Features
- **Daemonless Architecture:** No background processes. State is driven by filesystem and kernel syscalls
- **Dockerhub integration:** support for pulling multi-architecture OCI images form dockerhub
- **Overlay Filesystem:** Dynamic layered filesystem
- **Network Namespace Bridging:** Isolated container networking with custom IPAM, host to container communication, and NAT masquerading for internet access.
- **Cgroups v2 Resource Management:** dynamic limits for CPU, memory and PIDs (`--cpus`, `--memory`, `--cpuset`, `--pids`)
- **Port Forwarding:** Dynamic `iptables` routing for single or multiple ports
- **Volumes / Bind Mounts:** Persist data by mapping host directories directly into the container
- **Environment Injection:** Scrubbed and securely injected environment variables

## CLI Commands
- `o1 run [args] <image> <command>`: Spawn a new isolated container
- `o1 pull <image>`: Download an image from Docker Hub
- `o1 ps`: List all actively running containers
- `o1 stats`: Live dashboard of CPU & RAM utilization
- `o1 exec <id> <command>`: Inject a new process into a running container's namespace
- `o1 logs <id>`: Fetch standard output/error logs from a container
- `o1 stop <id>`: Send a graceful `SIGTERM` to a running container
- `o1 rm <id>`: Send `SIGKILL` and forcefully remove network/cgroup resources
- `o1 images`: List locally downloaded images
- `o1 rmi <image>`: Delete a local image


>Read the architecture at [architecture docs](./docs/core.md)
