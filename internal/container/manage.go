package container

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"text/tabwriter"
	"time"
)

// ps reads the state database and prints a formatted table of running containers
func Ps() {
	stateDir := "/var/lib/o1/state"
	files, err := os.ReadDir(stateDir)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Println("CONTAINER ID\tPID\tSTATUS\tCOMMAND")
			return
		}
		fmt.Printf("Error reading state: %v\n", err)
		return
	}

	// Initialize a tabwriter
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 4, ' ', 0)
	fmt.Fprintln(w, "CONTAINER ID\tPID\tIP ADDRESS\tSTATUS\tCOMMAND")

	for _, file := range files {
		if filepath.Ext(file.Name()) == ".json" {
			statePath := filepath.Join(stateDir, file.Name())
			data, err := os.ReadFile(statePath)
			if err != nil {
				continue
			}

			var state ContainerState
			if err := json.Unmarshal(data, &state); err != nil {
				continue
			}

			if err := syscall.Kill(state.PID, 0); err != nil {
				state.Status = "Dead"
			}

			fmt.Fprintf(w, "%s\t%d\t%s\t%s\t%s\n", state.ID, state.PID, state.IP, state.Status, state.Command)
		}
	}
	w.Flush()
}

// Stop gracefully kills the container process and wipes its data
func Stop(containerID string) {
	statePath := filepath.Join("/var/lib/o1/state", containerID+".json")
	data, err := os.ReadFile(statePath)
	if err != nil {
		fmt.Printf("Error: No such container: '%s'\n", containerID)
		return
	}

	var state ContainerState
	if err := json.Unmarshal(data, &state); err != nil {
		fmt.Printf("Error reading container state: %v\n", err)
		return
	}

	fmt.Printf("Stopping container %s (PID: %d)...\n", state.ID, state.PID)

	// send the SIGTERM signal to kill the process
	if err := syscall.Kill(state.PID, syscall.SIGTERM); err != nil {
		fmt.Printf("Warning: Failed to kill PID %d\n", state.PID)
	}

	// delete state json file
	os.Remove(statePath)

	// delete temp overlay workspace
	containerDir := filepath.Join("/var/lib/o1/containers", containerID)
	rootfsPath := filepath.Join(containerDir, "fs")
	syscall.Unmount(rootfsPath, syscall.MNT_DETACH) // forcefully unmount overlayFS
	os.RemoveAll(containerDir)

	fmt.Printf("Container %s successfully stopped and removed.\n", state.ID)
}

// Exec dumps a new process directly into a running container's namespace
func Exec(containerID string, userCmd []string) {
	statePath := filepath.Join("/var/lib/o1/state", containerID+".json")
	data, err := os.ReadFile(statePath)

	if err != nil {
		fmt.Printf("No such container running: %s\n", containerID)
		return
	}

	var state ContainerState
	if err := json.Unmarshal(data, &state); err != nil {
		fmt.Printf("Err reading container state: %v\n", err)
		return
	}

	if err := syscall.Kill(state.PID, 0); err != nil {
		fmt.Printf("Container %s is not running\n", containerID)
		return
	}

	SPid := strconv.Itoa(state.PID)
	args := []string{"-t", SPid, "-m", "-u", "-i", "-n", "-p"}
	args = append(args, userCmd...)
	cmd := exec.Command("nsenter", args...)

	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		fmt.Printf("Exec command failed: %v\n", err)
	}
}

// logs for debugging why my containers are crashing
func Logs(containerID string) {
	logPath := filepath.Join("/var/lib/o1/containers", containerID, "logs.txt")

	data, err := os.ReadFile(logPath)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Printf("Error: No logs found for container '%s'.\n", containerID)
		} else {
			fmt.Printf("Error reading logs: %v\n", err)
		}
		return
	}

	fmt.Printf("LOGS FOR : %s\n", containerID)
	fmt.Print(string(data))
}

// remove forcefully a container and cleans up all its resources
func Remove(containerID string) {
	stateDir := "/var/lib/o1/state"
	statePath := filepath.Join(stateDir, containerID+".json")

	data, err := os.ReadFile(statePath)
	if err != nil {
		fmt.Printf("Container %s not found.\n", containerID)
		return
	}

	var state ContainerState
	json.Unmarshal(data, &state)

	// kill process if still running
	if state.PID > 0 {
		fmt.Printf("Killing process with PID: %s\n", state.PID)
		syscall.Kill(state.PID, syscall.SIGKILL)
	}

	// clean the host's virtual ethernet cable
	if state.Veth != "" {
		fmt.Printf("Removing network interface: %s\n", state.Veth)
		exec.Command("ip", "link", "del", state.Veth).Run()
	}

	// delete the filesystem and logs
	containerDir := filepath.Join("/var/lib/o1/containers", containerID)
	rootfsPath := filepath.Join(containerDir, "fs")
	syscall.Unmount(rootfsPath, syscall.MNT_DETACH) // forcefully unmount overlayFS
	fmt.Printf("Deleting filesytem and logs at: %s\n", containerDir)
	os.RemoveAll(containerDir)

	// delete the cgroup
	cgPath := filepath.Join("/sys/fs/cgroup/o1-runtime", containerID)
	os.RemoveAll(cgPath)

	// remove from `o1 ps`
	os.Remove(statePath)

	// flush all iptable rules to prevent shadowing
	// safely remove only this specific container's masquerade rule
	// instead of deleting every single NAT rule on the host
	exec.Command("iptables", "-t", "nat", "-D", "POSTROUTING", "-s", state.IP+"/32", "-j", "MASQUERADE").Run()

	fmt.Printf("Container %s successfully removed!\n", containerID)
}

// read memory.current
func getMemoryUsage(containerID string) string {
	path := filepath.Join("/sys/fs/cgroup/o1-runtime", containerID, "memory.current")
	data, err := os.ReadFile(path)
	if err != nil {
		return "0.00 MB"
	}

	bytes, err := strconv.ParseFloat(strings.TrimSpace(string(data)), 64)
	if err != nil {
		return "0.00 MB"
	}

	mem := bytes / 1024.0 / 1024.0
	return fmt.Sprintf("%.2f MB", mem)
}

// read cpu.stat and extract total usage_usec counter
func getCpuUsageUsec(containerID string) int64 {
	path := filepath.Join("/sys/fs/cgroup/o1-runtime", containerID, "cpu.stat")
	file, err := os.Open(path)
	if err != nil {
		return 0
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		parts := strings.Fields(scanner.Text())
		if len(parts) == 2 && parts[0] == "usage_usec" {
			usec, _ := strconv.ParseInt(parts[1], 10, 64)
			return usec
		}
	}
	return 0
}

func Stats() {
	stateDir := "/var/lib/o1/state"

	lastCpuUsage := make(map[string]int64)
	lastTime := make(map[string]time.Time)

	for {
		fmt.Print("\033[2J\033[H") // clear the screen and move cursor to top left
		fmt.Printf("%-15s %-10s %-15s %-15s %-10s\n", "CONTAINER ID", "PID", "CPU %", "MEM USAGE", "STATUS")
		fmt.Println(strings.Repeat("-", 65))

		files, err := os.ReadDir(stateDir)
		if err != nil {
			fmt.Println("No containers running.")
			return
		}

		for _, file := range files {
			if filepath.Ext(file.Name()) == ".json" {
				data, err := os.ReadFile(filepath.Join(stateDir, file.Name()))
				if err != nil {
					continue
				}

				var state ContainerState
				json.Unmarshal(data, &state)

				if state.Status == "Dead" {
					continue
				}

				memStr := getMemoryUsage(state.ID)
				currentCpu := getCpuUsageUsec(state.ID)
				currentTime := time.Now()
				cpuPercent := 0.0

				if prevTime, exists := lastTime[state.ID]; exists {
					prevCpu := lastCpuUsage[state.ID]

					timeDelta := currentTime.Sub(prevTime).Microseconds()
					cpuDelta := currentCpu - prevCpu

					if timeDelta > 0 {
						cpuPercent = (float64(cpuDelta) / float64(timeDelta)) * 100.0
					}
				}

				lastCpuUsage[state.ID] = currentCpu
				lastTime[state.ID] = currentTime
				fmt.Printf("%-15s %-10d %-15.2f %-15s %-10s\n", state.ID[:8], state.PID, cpuPercent, memStr, state.Status)
			}
		}

		// 1 sec refresh rate
		time.Sleep(1 * time.Second)
	}
}

// calulate total size of a directory
func getDirSize(path string) (int64, error) {
	var size int64
	err := filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			size += info.Size()
		}
		return err
	})
	return size, err
}

// lists all downloaded OCI images
func Images() {
	imageDir := "/var/lib/o1/images"

	files, err := os.ReadDir(imageDir)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Println("No images downloaded yet.")
			return
		}
		fmt.Printf("Error reading images directory: %v\n", err)
		return
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 4, ' ', 0)
	fmt.Fprintln(w, "REPOSITORY\tTAG\tSIZE")

	imageCount := 0
	for _, file := range files {
		if file.IsDir() {

			// reverse the formatting we did in pull (library_ubuntu_latest -> ubuntu:latest)
			nameParts := strings.Split(file.Name(), "_")
			repo := ""
			tag := ""

			if len(nameParts) >= 2 {
				// Strip "library" from official images for cleaner output
				if nameParts[0] == "library" {
					repo = nameParts[1]
				} else {
					repo = nameParts[0] + "/" + nameParts[1]
				}
				tag = nameParts[len(nameParts)-1]
			} else {
				repo = file.Name()
				tag = "latest"
			}

			// calculate folder size
			path := filepath.Join(imageDir, file.Name())
			sizeBytes, _ := getDirSize(path)
			sizeMB := float64(sizeBytes) / 1024.0 / 1024.0

			fmt.Fprintf(w, "%s\t%s\t%.2f MB\n", repo, tag, sizeMB)
			imageCount++
		}
	}

	if imageCount == 0 {
		fmt.Println("REPOSITORY\tTAG\tSIZE")
	} else {
		w.Flush()
	}
}
