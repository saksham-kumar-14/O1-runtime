package main

import (
	"O1-runtime/internal/container"
	"fmt"
	"os"
)

func main() {
	if len(os.Args) < 3 {
		fmt.Println("Usage: o1 run <command>")
		os.Exit(1)
	}

	switch os.Args[1] {
	case "run":
		container.Run(os.Args[2:])
	case "child":
		container.Child(os.Args[2:])
	default:
		panic("Bad command")
	}
}
