package main

import (
	"O1-runtime/internal/container"
	"fmt"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: o1 [run|child|ps|stop] <args>")
		os.Exit(1)
	}

	switch os.Args[1] {
	case "run":
		container.Run(os.Args[2:])
	case "child":
		container.Child(os.Args[2:])
	case "ps":
		container.Ps()
	case "stop":
		if len(os.Args) < 3 {
			fmt.Println("Error: o1 stop requires a container ID")
			os.Exit(1)
		}
		container.Stop(os.Args[2])
	case "exec":
		if len(os.Args) < 4 {
			fmt.Println("Usage: o1 exec <container_id> <command>")
			os.Exit(1)
		}
		container.Exec(os.Args[2], os.Args[3:])
	case "logs":
		if len(os.Args) < 3 {
			fmt.Println("Usage: o1 logs <container_id>")
			os.Exit(1)
		}
		container.Logs(os.Args[2])
	case "rm":
		if len(os.Args) < 3 {
			fmt.Println("Usage: o1 rm <container_id>")
			os.Exit(1)
		}
		container.Remove(os.Args[2])
	case "stats":
		// root is required to read cgroup file
		if os.Geteuid() != 0 {
			fmt.Println("Please run as root `sudo o1 stats`")
			os.Exit(1)
		}
		container.Stats()
	default:
		panic("Bad command")
	}
}
