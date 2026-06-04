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
	default:
		panic("Bad command")
	}
}
