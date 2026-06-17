package container

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func copyDir(src string, dst string) error {
	return exec.Command("cp", "-a", src+"/.", dst+"/").Run()
}

func Build(dockerfilePath string, targetImage string) {
	fmt.Printf("Parsing Dockerfile at %s...\n", dockerfilePath)

	file, err := os.Open(dockerfilePath)
	if err != nil {
		fmt.Printf("Error opening Dockerfile: %v\n", err)
		os.Exit(1)
	}
	defer file.Close()

	var baseImage string
	var runCommands []string
	var finalCmd []string

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.Fields(line)
		instruction := strings.ToUpper(parts[0])

		switch instruction {
		case "FROM":
			baseImage = parts[1]
		case "RUN":
			runCommands = append(runCommands, strings.Join(parts[1:], " "))
		case "CMD":
			// Basic parser for ["python3", "app.py"]
			rawCmd := strings.Join(parts[1:], " ")
			rawCmd = strings.ReplaceAll(rawCmd, "[", "")
			rawCmd = strings.ReplaceAll(rawCmd, "]", "")
			rawCmd = strings.ReplaceAll(rawCmd, "\"", "")
			finalCmd = strings.Split(rawCmd, ",")
			for i := range finalCmd {
				finalCmd[i] = strings.TrimSpace(finalCmd[i])
			}
		}
	}

	if baseImage == "" {
		fmt.Println("Error: Dockerfile must start with a FROM instruction.")
		os.Exit(1)
	}

	fmt.Printf("Preparing base image '%s'...\n", baseImage)

	// Format the base image name like library_alpine_latest
	baseName := baseImage
	if !strings.Contains(baseName, ":") {
		baseName += "_latest"
	}
	if !strings.Contains(baseName, "/") {
		baseName = "library_" + baseName
	}
	baseDir := filepath.Join("/var/lib/o1/images", strings.ReplaceAll(baseName, "/", "_"))

	if _, err := os.Stat(baseDir); os.IsNotExist(err) {
		fmt.Printf("Base image not found locally. Pulling %s...\n", baseImage)
		Pull(baseImage)
	}

	targetName := targetImage
	if !strings.Contains(targetName, ":") {
		targetName += "_latest"
	}
	if !strings.Contains(targetName, "/") {
		targetName = "library_" + targetName
	}
	targetDir := filepath.Join("/var/lib/o1/images", strings.ReplaceAll(targetName, "/", "_"))

	os.RemoveAll(targetDir)
	os.MkdirAll(targetDir, 0755)
	fmt.Printf("Step 3: Cloning base filesystem to %s...\n", targetDir)
	copyDir(baseDir, targetDir)

	for i, cmdStr := range runCommands {
		fmt.Printf("Step 4.%d: Running [%s]...\n", i+1, cmdStr)

		// boot a container using our NEW image as the base.
		// the container will make changes in its temporary 'upper' directory.
		cmd := exec.Command("/proc/self/exe", "run", targetImage, "/bin/sh", "-c", cmdStr)

		// Capture output to find the Container ID
		output, err := cmd.CombinedOutput()
		if err != nil {
			fmt.Printf("Build failed during RUN command: %v\nOutput:\n%s\n", err, string(output))
			os.Exit(1)
		}

		// extract the container ID from the output
		var containerID string
		for _, line := range strings.Split(string(output), "\n") {
			if strings.HasPrefix(line, "ID: ") {
				containerID = strings.TrimSpace(strings.TrimPrefix(line, "ID: "))
				break
			}
		}

		if containerID != "" {
			// copy the modified files from the container's upperdir into our new image
			upperDir := filepath.Join("/var/lib/o1/containers", containerID, "upper")
			copyDir(upperDir, targetDir)
			Remove(containerID)
		}
	}

	// write config manifest
	fmt.Println("Step 5: Writing OCI Configuration Manifest...")

	configPath := filepath.Join(targetDir, "config.json")
	var ociConfig OCIConfig

	oldConfigData, _ := os.ReadFile(filepath.Join(baseDir, "config.json"))
	json.Unmarshal(oldConfigData, &ociConfig)

	// Override with the new CMD from our Dockerfile
	ociConfig.Config.Entrypoint = []string{} // Clear entrypoint to avoid conflicts
	ociConfig.Config.Cmd = finalCmd

	newConfigData, _ := json.MarshalIndent(ociConfig, "", "  ")
	os.WriteFile(configPath, newConfigData, 0644)

	fmt.Printf("\nSuccessfully built image: %s\n", targetImage)
}
