// parent process running on the host OS
// only job is to make children

package container

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"
)

func Run(args []string) {
	cmdArgs := append([]string{"child"}, args...)
	cmd := exec.Command("/proc/self/exe", cmdArgs...)

	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	cmd.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags: syscall.CLONE_NEWUTS | syscall.CLONE_NEWPID | syscall.CLONE_NEWNS,
	}

	if err := cmd.Start(); err != nil {
		fmt.Printf("HOST: container failed to start, error: %v\n", err)
		os.Exit(1)
	}

	applyCgroups(cmd.Process.Pid)

	if err := cmd.Wait(); err != nil {
		fmt.Printf("HOST: container exited with error: %v\n", err)
		os.Exit(1)
	}
}

func applyCgroups(pid int) {
	cgPath := "/sys/fs/cgroup"
	dir := filepath.Join(cgPath, "o1-runtime")

	if err := os.MkdirAll(dir, 0755); err != nil {
		panic(fmt.Sprintf("Error creating cgroup directory: %v", err))
	}

	pidsMaxPath := filepath.Join(dir, "pids.max")
	if err := os.WriteFile(pidsMaxPath, []byte("20"), 0700); err != nil {
		panic(fmt.Sprintf("Failed to write pids.max: %v", err))
	}
	memoryMaxPath := filepath.Join(dir, "memory.max")
	if err := os.WriteFile(memoryMaxPath, []byte("52428800"), 0700); err != nil {
		panic(fmt.Sprintf("Failed to write memory.max: %v", err))
	}

	procsPath := filepath.Join(dir, "cgroup.procs")
	if err := os.WriteFile(procsPath, []byte(strconv.Itoa(pid)), 0700); err != nil {
		panic(fmt.Sprintf("Failed to write cgroup.procs: %v", err))
	}
}
