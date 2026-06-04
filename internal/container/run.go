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
		Cloneflags: syscall.CLONE_NEWUTS | syscall.CLONE_NEWPID | syscall.CLONE_NEWNS | syscall.CLONE_NEWNET,
	}

	if err := cmd.Start(); err != nil {
		fmt.Printf("HOST: container failed to start, error: %v\n", err)
		os.Exit(1)
	}

	applyCgroups(cmd.Process.Pid)
	setupNetwork(cmd.Process.Pid)

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

func runCmd(name string, args ...string) {
	cmd := exec.Command(name, args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		if args[0] != "deg" {
			fmt.Printf("Network config warning: %v\nOutput: %v\n", err, out)
		}
	}
}

func setupNetwork(pid int) {
	pidStr := strconv.Itoa(pid)

	// create new network namespace symlink
	os.MkdirAll("/var/run/netns", 0755)
	netnsPath := filepath.Join("/var/run/netns", pidStr)
	os.Remove(netnsPath) // Clean up just in case
	if err := os.Symlink(filepath.Join("/proc", pidStr, "ns", "net"), netnsPath); err != nil {
		fmt.Printf("Warning: Failed to symlink netns: %v\n", err)
	}
	defer os.Remove(netnsPath)

	// clean up old interface
	exec.Command("ip", "link", "del", "veth0").Run()

	// create virtual ethernet pair
	runCmd("ip", "link", "add", "dev", "veth0", "type", "veth", "peer", "name", "veth1")

	// move veth1 to container's network namespace
	runCmd("ip", "link", "set", "dev", "veth1", "netns", pidStr)

	// configure host (veth0)
	runCmd("ip", "addr", "add", "10.0.0.1/24", "dev", "veth0")
	runCmd("ip", "link", "set", "dev", "veth0", "up")

	// configure container (veth1)
	runCmd("nsenter", "-t", pidStr, "-n", "ip", "addr", "add", "10.0.0.2/24", "dev", "veth1")
	runCmd("nsenter", "-t", pidStr, "-n", "ip", "link", "set", "dev", "veth1", "up")
	runCmd("nsenter", "-t", pidStr, "-n", "ip", "link", "set", "dev", "lo", "up")
	runCmd("nsenter", "-t", pidStr, "-n", "ip", "route", "add", "default", "via", "10.0.0.1")

	// port forwarding
	// allow linux to route localhost traffic through our virtual cable
	runCmd("sysctl", "-w", "net.ipv4.conf.veth0.route_localnet=1")
	// allow container to reach the internet
	runCmd("iptables", "-t", "nat", "-A", "POSTROUTING", "-s", "10.0.0.2/32", "-j", "MASQUERADE")
	// route traffic from port 8080 into the container
	runCmd("iptables", "-t", "nat", "-A", "PREROUTING", "-p", "tcp", "--dport", "8080", "-j", "DNAT", "--to-destination", "10.0.0.2:80")
	runCmd("iptables", "-t", "nat", "-A", "OUTPUT", "-p", "tcp", "--dport", "8080", "-j", "DNAT", "--to-destination", "10.0.0.2:80")
	// disguise the packet so the container knows exactly how to reply
	runCmd("iptables", "-t", "nat", "-A", "POSTROUTING", "-d", "10.0.0.2/32", "-p", "tcp", "--dport", "80", "-j", "MASQUERADE")
}
