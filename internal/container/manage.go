package container

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"
	"text/tabwriter"
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

	fmt.Printf("LOGS FOR : %sn", containerID)
	fmt.Print(string(data))
}
