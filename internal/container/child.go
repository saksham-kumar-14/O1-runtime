package container

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
)

func Child(args []string) {
	rootfsPath := "/fs"
	oldfsPath := "/fs/oldfs"

	if err := syscall.Sethostname([]byte("o1-container")); err != nil {
		fmt.Printf("Error setting the hostname: %v\n", err)
		os.Exit(1)
	}

	if err := syscall.Mount("", "/", "", syscall.MS_PRIVATE|syscall.MS_REC, ""); err != nil {
		panic(fmt.Sprintf("Error making mounts private: %v", err))
	}

	if err := syscall.Mount(rootfsPath, rootfsPath, "", syscall.MS_BIND, ""); err != nil {
		panic(fmt.Sprintf("Error bind mounting rootfs: %v", err))
	}

	if err := os.MkdirAll(oldfsPath, 0700); err != nil {
		panic(fmt.Sprintf("Error creating oldfs directory: %v", err))
	}

	resolvPath := filepath.Join(rootfsPath, "etc", "resolv.conf")
	if err := os.WriteFile(resolvPath, []byte("nameserver 8.8.8.8\n"), 0644); err != nil {
		fmt.Printf("Warning: Failed to setup DNS: %v\n", err)
	}

	if err := syscall.PivotRoot(rootfsPath, oldfsPath); err != nil {
		panic(fmt.Sprintf("Error pivoting root: %v", err))
	}

	if err := os.Chdir("/"); err != nil {
		panic(fmt.Sprintf("Error changing dir to /: %v", err))
	}

	if err := syscall.Mount("proc", "proc", "proc", 0, ""); err != nil {
		panic(fmt.Sprintf("Error mounting proc: %v", err))
	}

	syscall.Unmount("/oldfs", syscall.MNT_DETACH)
	os.RemoveAll("/oldfs")

	cmd := exec.Command(args[0], args[1:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Printf("Container: Command failed: %v\n", err)
		os.Exit(1)
	}

	syscall.Unmount("/proc", 0)
}
