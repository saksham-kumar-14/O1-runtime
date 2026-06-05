# O1 runtime
`O1` is a lightweight runtime built in Go

## Features
- Lightweight
- Overlay Filesystem
- Network Bridge: now my laptop can communicate with the containers running
- `o1 ps`: list of running containers
- `o1 stop <CONTAINER_ID>`: stop the container
- `o1 exec`
- `o1 logs`
- Dynamic Networking
- IP allocators for the containers
- Bind Mount/Volumes: allow to map folder from host directly into the container
  - *Concurrency Race condition:* 
    - Doing a light task such as writing string to a text file takes way less time than my host go process to create virtual ethernet cable, attach it to bridge and call `nsenter` to inject it to the container
    - By the time host is done with this network stuff, the container finishes the light task and shut down
    - That's why `nsenter` throws error `No such process found!` ------> but there is no process!
- [ ] Environment Variables: config data for containers
- [ ] Multiple port mapping
- [ ] Named containers + calling them by unique initials
- [ ] Dynamic resource allocation: as of now limits are hardcoded in `applyCGroups()`
