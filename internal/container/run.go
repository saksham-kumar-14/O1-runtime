// parent process running on the host OS
// only job is to make children

package container

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

type ContainerState struct {
	ID      string `json:"id"`
	PID     int    `json:"pid"`
	Status  string `json:"status"`
	Command string `json:"command"`
}

func generateID() string {
	b := make([]byte, 4)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func Run(args []string) {

	containerID := generateID()

	// dynamic port parsing
	hostPort := ""
	containerPort := ""
	var execArgs []string

	for i := 0; i < len(args); i++ {
		if args[i] == "-p" && i+1 < len(args) {
			ports := strings.Split(args[i+1], ":")
			if len(ports) == 2 {
				hostPort = ports[0]
				containerPort = ports[1]
			}
			i++
		} else {
			execArgs = append(execArgs, args[i])
		}
	}

	cmdArgs := append([]string{"child", containerID}, execArgs...)
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
	setupNetwork(cmd.Process.Pid, hostPort, containerPort)

	// save to state database
	state := ContainerState{
		ID:      containerID,
		PID:     cmd.Process.Pid,
		Status:  "Running",
		Command: strings.Join(args, " "),
	}

	stateBytes, _ := json.Marshal(state)
	os.MkdirAll("/var/lib/o1/state", 0755)
	statePath := filepath.Join("/var/lib/o1/state", containerID+".json")
	os.WriteFile(statePath, stateBytes, 0644)

	fmt.Printf("Container started successfully!\nID: %s\nPID: %d\n", containerID, cmd.Process.Pid)
	os.Exit(0)
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

func setupNetwork(pid int, hostPort string, containerPort string) {
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

	// dynamic routing
	// setup port forwarding if port is provided
	if hostPort != "" && containerPort != "" {
		runCmd("sysctl", "-w", "net.ipv4.conf.veth0.route_localnet=1")
		runCmd("iptables", "-t", "nat", "-A", "POSTROUTING", "-s", "10.0.0.2/32", "-j", "MASQUERADE")

		// inject the dynamic variables into iptables
		runCmd("iptables", "-t", "nat", "-A", "PREROUTING", "-p", "tcp", "--dport", hostPort, "-j", "DNAT", "--to-destination", "10.0.0.2:"+containerPort)
		runCmd("iptables", "-t", "nat", "-A", "OUTPUT", "-p", "tcp", "--dport", hostPort, "-j", "DNAT", "--to-destination", "10.0.0.2:"+containerPort)
		runCmd("iptables", "-t", "nat", "-A", "POSTROUTING", "-d", "10.0.0.2/32", "-p", "tcp", "--dport", containerPort, "-j", "MASQUERADE")
	}
}
