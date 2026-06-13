# O1 runtime
`O1` is a lightweight runtime built in Go

## Features
- Lightweight
- Overlay Filesystem
- Network Bridge: now my laptop can communicate with the containers running
- `o1 ps`: list of running containers
- `o1 stop <CONTAINER_ID>`: stop the container
- `o1 exec <CONTAINER_ID> <COMMAND>`
- `o1 logs <CONTAINER_ID>`
- `o1 rm <CONTAINER_ID>`
- Port Mapping: `-p <host>:<container>`
- IP allocators for the containers
- Bind Mount/Volumes: allow to map folder from host directly into the container `-v <host>:<container>`
- Environment Variables: config data for containers `-e <KEY=VALUE>`
- Multiple port mapping
- Dynamic resource allocation: as of now limits are hardcoded in `applyCGroups()`
- [ ] Named containers + calling them by unique initials

>Read the architecture at [architecture docs](./docs/core.md)
