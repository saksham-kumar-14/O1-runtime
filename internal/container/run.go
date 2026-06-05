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
	IP      string `json:"ip"`
	Veth    string `json:"veth"`
}

func getAvailableIP() string {
	stateDir := "/var/lib/o1/state"
	files, err := os.ReadDir(stateDir)
	if err != nil {
		panic(fmt.Sprintf("IPAM error: %v\n", err))
	}

	doneIPs := make(map[string]bool)

	for _, file := range files{
		if filepath.Ext(file.Name()) == ".json"{
			data, err := os.ReadFile(filepath.Join(stateDir, file.Name()))
			if err == nil{
				var state ContainerState
				json.Unmarshal(data, &state)
				if state.IP != ""{
					doneIPs[state.IP] = true
				}
			}
		}
	}

	for i := 2; i <= 254; i++ {
		ip := fmt.Sprintf("10.0.0.%d", i)
		if !doneIPs[ip] {
			return ip
		}
	}

	panic("IPAM Error: No available IP addresses in subnet!")
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

	containerIP := getAvailableIP()
	hostVeth := "veth-" + containerID[:4]

	setupNetwork(cmd.Process.Pid, hostPort, containerPort, containerIP, hostVeth)

	// save to state database
	state := ContainerState{
		ID:      containerID,
		PID:     cmd.Process.Pid,
		Status:  "Running",
		Command: strings.Join(args, " "),
		IP:      containerIP,
		Veth:    hostVeth,
	}

	stateBytes, _ := json.Marshal(state)
	os.MkdirAll("/var/lib/o1/state", 0755)
	statePath := filepath.Join("/var/lib/o1/state", containerID+".json")
	os.WriteFile(statePath, stateBytes, 0644)

	fmt.Printf("Container started successfully!\nID: %s\nPID: %d\nIP: %s\n", containerID, cmd.Process.Pid, containerIP)
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
	if err := os.WriteFile(memoryMaxPath, []byte("536870912"), 0700); err != nil {
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

func setupNetwork(pid int, hostPort string, containerPort string, containerIP string, hostVeth string) {
	pidStr := strconv.Itoa(pid)

	// ensure the core O1 Network Bridge exists
	exec.Command("ip", "link", "add", "name", "o1-br0", "type", "bridge").Run()
	exec.Command("ip", "addr", "add", "10.0.0.1/24", "dev", "o1-br0").Run()
	exec.Command("ip", "link", "set", "dev", "o1-br0", "up").Run()

	// enable bridge firewall forwarding
	runCmd("sysctl", "-w", "net.ipv4.ip_forward=1")
	runCmd("iptables", "-A", "FORWARD", "-i", "o1-br0", "-j", "ACCEPT")
	runCmd("iptables", "-A", "FORWARD", "-o", "o1-br0", "-j", "ACCEPT")

	// create network namespace symlink
	os.MkdirAll("/var/run/netns", 0755)
	netnsPath := filepath.Join("/var/run/netns", pidStr)
	os.Remove(netnsPath)
	if err := os.Symlink(filepath.Join("/proc", pidStr, "ns", "net"), netnsPath); err != nil {
		fmt.Printf("Warning: Failed to symlink netns: %v\n", err)
	}
	defer os.Remove(netnsPath)

	// a unique temporary name for the child interface to avoid host-side collisions
	childVeth := "veth-ch-" + pidStr

	exec.Command("ip", "link", "del", hostVeth).Run()
	exec.Command("ip", "link", "del", childVeth).Run()

	// create virtual ethernet pair using unique temporary names
	runCmd("ip", "link", "add", "dev", hostVeth, "type", "veth", "peer", "name", childVeth)

	// the host side of the cable goes directly into our virtual switch bridge
	runCmd("ip", "link", "set", "dev", hostVeth, "master", "o1-br0")
	runCmd("ip", "link", "set", "dev", hostVeth, "up")

	// move the unique child interface into the container's network namespace
	runCmd("ip", "link", "set", "dev", childVeth, "netns", pidStr)

	// configure the interface inside the container namespace using the bridge gateway
	runCmd("nsenter", "-t", pidStr, "-n", "ip", "addr", "add", containerIP+"/24", "dev", childVeth)
	runCmd("nsenter", "-t", pidStr, "-n", "ip", "link", "set", "dev", childVeth, "up")
	runCmd("nsenter", "-t", pidStr, "-n", "ip", "link", "set", "dev", "lo", "up")
	runCmd("nsenter", "-t", pidStr, "-n", "ip", "route", "add", "default", "via", "10.0.0.1")

	// dynamic port Forwarding routed through the bridge
	if hostPort != "" && containerPort != "" {
		// allow linux to route localhost traffic through our network bridge
		runCmd("sysctl", "-w", "net.ipv4.conf.o1-br0.route_localnet=1")

		// allow container to reach the outside internet via NAT masquerade
		runCmd("iptables", "-t", "nat", "-A", "POSTROUTING", "-s", containerIP+"/32", "-j", "MASQUERADE")

		// route incoming host traffic straight to the dynamic container IP
		runCmd("iptables", "-t", "nat", "-A", "PREROUTING", "-p", "tcp", "--dport", hostPort, "-j", "DNAT", "--to-destination", containerIP+":"+containerPort)
		runCmd("iptables", "-t", "nat", "-A", "OUTPUT", "-p", "tcp", "--dport", hostPort, "-j", "DNAT", "--to-destination", containerIP+":"+containerPort)
		runCmd("iptables", "-t", "nat", "-A", "POSTROUTING", "-d", containerIP+"/32", "-p", "tcp", "--dport", containerPort, "-j", "MASQUERADE")
	}
}
