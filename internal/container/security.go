package container

import (
	"fmt"
	"syscall"

	seccomp "github.com/seccomp/libseccomp-golang"
)

func setupSeccomp() error {
	filter, err := seccomp.NewFilter(seccomp.ActAllow)
	if err != nil {
		return fmt.Errorf("failed to create seccomp filter: %v", err)
	}
	defer filter.Release()

	blockedSyscalls := []string{
		"ptrace",
		"mount",
		"unmount2",
		"unshare",
		"kexec_load",
		"bpf",
	}

	for _, call := range blockedSyscalls {
		syscallID, err := seccomp.GetSyscallFromName(call)
		if err != nil {
			continue
		}

		err = filter.AddRule(syscallID, seccomp.ActErrno.SetReturnCode(int16(syscall.EPERM)))
		if err != nil {
			return fmt.Errorf("failed to add seccomp rule for %s: %v", call, err)
		}
	}

	if err := filter.Load(); err != nil {
		return fmt.Errorf("failed to load seccomp filter: %v", err)
	}
	return nil
}
