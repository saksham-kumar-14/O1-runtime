package container

import (
	"fmt"
	"os"
	"os/exec"
)

func Child(args []string) {
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Printf("Container: Command failed: %v\n", err)
		os.Exit(1)
	}
}
