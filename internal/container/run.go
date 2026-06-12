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

type PortMapping struct {
	HostPort      string
	ContainerPort string
}

func getAvailableIP() string {
	stateDir := "/var/lib/o1/state"
	files, err := os.ReadDir(stateDir)
	if err != nil {
		panic(fmt.Sprintf("IPAM error: %v\n", err))
	}

	doneIPs := make(map[string]bool)

	for _, file := range files {
		if filepath.Ext(file.Name()) == ".json" {
			data, err := os.ReadFile(filepath.Join(stateDir, file.Name()))
			if err == nil {
				var state ContainerState
				json.Unmarshal(data, &state)
				if state.IP != "" {
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
	// hostPort := ""
	// containerPort := ""
	volume := ""

	memoryLimit := "" // for dynamic resource allocation
	pidsLimit := ""   // for dynamic resource allocation

	var envVars []string // for env variables `-e`
	var execArgs []string
	var ports []PortMapping // for multiple ports

	for i := 0; i < len(args); i++ {
		if args[i] == "-p" && i+1 < len(args) {
			p := strings.Split(args[i+1], ":")
			if len(p) == 2 {
				ports = append(ports, PortMapping{HostPort: p[0], ContainerPort: p[1]})
			}
			i++
		} else if args[i] == "-v" && i+1 < len(args) {
			volume = args[i+1]
			i++
		} else if args[i] == "-e" && i+1 < len(args) {
			envVars = append(envVars, args[i+1])
			i++
		} else if args[i] == "--memory" && i+1 < len(args) {
			memoryLimit = args[i+1]
			i++
		} else if args[i] == "--pids" && i+1 < len(args) {
			pidsLimit = args[i+1]
			i++
		} else {
			execArgs = append(execArgs, args[i])
		}
	}

	cmdArgs := append([]string{"child", containerID}, execArgs...)
	cmd := exec.Command("/proc/self/exe", cmdArgs...)

	cmd.Env = append(os.Environ(), "O1_VOLUME="+volume) // put volume data in child process
	cmd.Env = append(cmd.Env, envVars...)               // append environment variables as environment variables

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

	applyCgroups(cmd.Process.Pid, containerID, memoryLimit, pidsLimit)

	containerIP := getAvailableIP()
	hostVeth := "veth-" + containerID[:4]

	setupNetwork(cmd.Process.Pid, ports, containerIP, hostVeth)

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

func applyCgroups(pid int, containerID string, memoryLimit string, pidsLimit string) {
	cgPath := "/sys/fs/cgroup"
	dir := filepath.Join(cgPath, "o1-runtime")

	if err := os.MkdirAll(dir, 0755); err != nil {
		panic(fmt.Sprintf("Error creating cgroup directory: %v", err))
	}

	pidsMaxPath := filepath.Join(dir, "pids.max")
	if err := os.WriteFile(pidsMaxPath, []byte(pidsLimit), 0700); err != nil {
		panic(fmt.Sprintf("Failed to write pids.max: %v", err))
	}
	memoryMaxPath := filepath.Join(dir, "memory.max")
	if err := os.WriteFile(memoryMaxPath, []byte(memoryLimit), 0700); err != nil {
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

func setupNetwork(pid int, ports []PortMapping, containerIP string, hostVeth string) {
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

	// for internet access
	// allow linux to route localhost traffic through our network bridge
	runCmd("sysctl", "-w", "net.ipv4.conf.o1-br0.route_localnet=1")

	// allow container to reach the outside internet via NAT masquerade
	runCmd("iptables", "-t", "nat", "-A", "POSTROUTING", "-s", containerIP+"/32", "-j", "MASQUERADE")

	// dyanmic port Forwrding routed through the bridge
	if len(ports) > 0 {
		// route incoming host traffic straight to the dynamic container IP
		for _, p := range ports {
			runCmd("iptables", "-t", "nat", "-A", "PREROUTING", "-p", "tcp", "--dport", p.HostPort, "-j", "DNAT", "--to-destination", containerIP+":"+p.ContainerPort)
			runCmd("iptables", "-t", "nat", "-A", "OUTPUT", "-p", "tcp", "--dport", p.HostPort, "-j", "DNAT", "--to-destination", containerIP+":"+p.ContainerPort)
			runCmd("iptables", "-t", "nat", "-A", "POSTROUTING", "-d", containerIP+"/32", "-p", "tcp", "--dport", p.ContainerPort, "-j", "MASQUERADE")
		}
	}
}
