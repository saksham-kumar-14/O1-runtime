package container

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"
)

func Child(args []string) {

	// setting hostname for this specific container
	if err := syscall.Sethostname([]byte("o1-container")); err != nil {
		fmt.Print("Error setting the hostname: %v\n", err)
		os.Exit(1)
	}

	cmd := exec.Command(args[0], args[1:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Printf("Container: Command failed: %v\n", err)
		os.Exit(1)
	}
}
