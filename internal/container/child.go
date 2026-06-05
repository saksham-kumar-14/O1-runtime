package container

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
)

func Child(args []string) {

	if len(args) < 2 {
		panic("Child requires container ID and command")
	}
	containerID := args[0]
	execArgs := args[1:]

	imageDir := "/var/lib/o1/images/default"

	containerDir := filepath.Join("/var/lib/o1/containers", containerID) // create a unique overlayFS dir for this container
	upperDir := filepath.Join(containerDir, "upper")
	workDir := filepath.Join(containerDir, "work")

	rootfsPath := filepath.Join(containerDir, "fs")
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

	logPath := filepath.Join(containerDir, "logs.txt")
	logFile, err := os.Create(logPath)
	if err != nil {
		panic(fmt.Sprintf("Failed to create log file: %v", err))
	}
	defer logFile.Close()

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

	// persistant storage
	vol := os.Getenv("O1_VOLUME")
	if vol != "" {
		temp := strings.Split(vol, ":")
		if len(temp) == 2 {
			hostPath := temp[0]
			containerPath := filepath.Join(rootfsPath, temp[1])

			// just for sanity check kind of
			if err := os.MkdirAll(hostPath, 0777); err != nil {
				panic(fmt.Sprintf("Host path error: %v", err))
			}
			if err := os.MkdirAll(containerPath, 0777); err != nil {
				panic(fmt.Sprintf("Container path error: %v", err))
			}

			// MS_REC ensures if there are mounts inside the host folder, they carry over too
			err := syscall.Mount(hostPath, containerPath, "bind", syscall.MS_BIND|syscall.MS_REC, "")
			if err != nil {
				panic(fmt.Sprintf("Failed to bind mount volume: %v\n", err))
			}
		}
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

	cmd := exec.Command(execArgs[0], execArgs[1:]...)

	cmd.Stdin = os.Stdin
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	if err := cmd.Run(); err != nil {
		os.Exit(1)
	}

	syscall.Unmount("/proc", 0)
}
