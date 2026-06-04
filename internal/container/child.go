package container

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
)

func Child(args []string) {

	imageDir := "/var/lib/o1/images/default"
	containerDir := "/var/lib/o1/containers/temp"
	upperDir := filepath.Join(containerDir, "upper")
	workDir := filepath.Join(containerDir, "work")

	rootfsPath := "/fs"
	oldfsPath := filepath.Join(rootfsPath, "oldfs")

	if err := syscall.Sethostname([]byte("o1-container")); err != nil {
		fmt.Printf("Error setting the hostname: %v\n", err)
		os.Exit(1)
	}

	if err := syscall.Mount("", "/", "", syscall.MS_PRIVATE|syscall.MS_REC, ""); err != nil {
		panic(fmt.Sprintf("Error making mounts private: %v", err))
	}

	os.RemoveAll(containerDir)
	os.MkdirAll(upperDir, 0777)
	os.MkdirAll(workDir, 0777)
	os.MkdirAll(rootfsPath, 0777)

	// base image sanity check
	if _, err := os.Stat(imageDir); os.IsNotExist(err) {
		fmt.Printf("Container Error: Base image not found at %s\n", imageDir)
		os.Exit(1)
	}

	// mount overlay
	opts := fmt.Sprintf("lowerdir=%s,upperdir=%s,workdir=%s", imageDir, upperDir, workDir)
	if err := syscall.Mount("overlay", rootfsPath, "overlay", 0, opts); err != nil {
		panic(fmt.Sprintf("Error mounting overlayfs: %v", err))
	}

	// give DNS to merged fs
	resolvPath := filepath.Join(rootfsPath, "etc", "resolv.conf")
	os.WriteFile(resolvPath, []byte("nameserver 8.8.8.8\n"), 0644)

	// pivot root into merged fs
	if err := os.MkdirAll(oldfsPath, 0700); err != nil {
		panic(fmt.Sprintf("Error creating oldfs directory: %v", err))
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
