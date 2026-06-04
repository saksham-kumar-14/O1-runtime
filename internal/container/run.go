// parent process running on the host OS
// only job is to make children

package container

import (
	"fmt"
	"os"
	"os/exec"
)

func Run(args []string) {
	cmdArgs := append([]string{"child"}, args...)
	cmd := exec.Command("/proc/self/exe", cmdArgs...)

	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		fmt.Printf("Host container exited with error: %v\n", err)
		os.Exit(1)
	}
}
