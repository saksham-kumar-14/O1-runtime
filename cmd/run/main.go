// main.go
package main

import (
	"O1-runtime/internal/container"
	"fmt"
	"os"
)

func printHelp() {
	fmt.Println("o1 - A lightweight Linux container engine")
	fmt.Println("\nUsage:")
	fmt.Println("  sudo o1 <command> [arguments]")
	fmt.Println("\nCommands:")
	fmt.Println("  run <image> <cmd>  Create and run a new container")
	fmt.Println("  ps                 List running containers")
	fmt.Println("  exec <id> <cmd>    Run a command in an existing container")
	fmt.Println("  logs <id>          Fetch the logs of a container")
	fmt.Println("  stats              Display a live stream of container resource usage")
	fmt.Println("  stop <id>          Gracefully stop a running container")
	fmt.Println("  rm <id>            Force remove a container and its resources")
	fmt.Println("  pull <image>       Download an image from Docker Hub")
	fmt.Println("  images             List downloaded images")
	fmt.Println("  rmi <image>        Remove a downloaded image")
	fmt.Println("  build <file> <name> Build a custom image from a Dockerfile")
	fmt.Println("\nInternal Commands:")
	fmt.Println("  child              (Do not call manually) Init process inside namespace")
}

func main() {
	if len(os.Args) < 2 {
		printHelp()
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
			fmt.Println("Usage: sudo o1 stop <container_id>")
			os.Exit(1)
		}
		container.Stop(os.Args[2])
	case "exec":
		if len(os.Args) < 4 {
			fmt.Println("Usage: sudo o1 exec <container_id> <command>")
			os.Exit(1)
		}
		container.Exec(os.Args[2], os.Args[3:])
	case "logs":
		if len(os.Args) < 3 {
			fmt.Println("Usage: sudo o1 logs <container_id>")
			os.Exit(1)
		}
		container.Logs(os.Args[2])
	case "rm":
		if len(os.Args) < 3 {
			fmt.Println("Usage: sudo o1 rm <container_id>")
			os.Exit(1)
		}
		container.Remove(os.Args[2])
	case "stats":
		if os.Geteuid() != 0 {
			fmt.Println("Please run as root: sudo o1 stats")
			os.Exit(1)
		}
		container.Stats()
	case "pull":
		if len(os.Args) < 3 {
			fmt.Println("Usage: sudo o1 pull <image>")
			os.Exit(1)
		}
		container.Pull(os.Args[2])
	case "images":
		container.Images()
	case "rmi":
		if len(os.Args) < 3 {
			fmt.Println("Usage: sudo o1 rmi <image>")
			os.Exit(1)
		}
		container.Rmi(os.Args[2])
	case "build":
		if len(os.Args) < 4 {
			fmt.Println("Usage: sudo o1 build <path/to/Dockerfile> <new_image_name>")
			os.Exit(1)
		}
		container.Build(os.Args[2], os.Args[3])
	case "help", "--help", "-h":
		printHelp()
	default:
		fmt.Printf("o1: '%s' is not an o1 command.\n", os.Args[1])
		printHelp()
		os.Exit(1)
	}
}
