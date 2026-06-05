# O1 runtime
`O1` is a lightweight runtime built in Go

## Features
- Lightweight
- Overlay Filesystem
- Network Bridge: now my laptop can communicate with the containers running
- `o1 ps`: list of running containers
- `o1 stop <CONTAINER_ID>`: stop the container
- `o1 exec`
- Dynamic Networking
- IP allocators for the containers
- [ ] `o1 logs`
- [ ] Bind Mount/Volumes: allow to map folder from host directly into the container
- [ ] Environment Variables: config data for containers
- [ ] Multiple port mapping
- [ ] Named containers + calling them by unique initials
- [ ] Dynamic resource allocation: as of now limits are hardcoded in `applyCGroups()`
