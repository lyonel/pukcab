package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"strconv"
)

func remotecommand(arg ...string) *exec.Cmd {
	os.Setenv("SSH_CLIENT", "")
	os.Setenv("SSH_CONNECTION", "")

	if cfg.Server != "" {
		cmd := []string{"-q", "-C", "-oBatchMode=yes", "-oStrictHostKeyChecking=no", "-oUserKnownHostsFile=/dev/null"}
		if cfg.User != "" {
			cmd = append(cmd, "-l", cfg.User)
		}
		if cfg.Port > 0 {
			cmd = append(cmd, "-p", strconv.Itoa(cfg.Port))
		}
		cmd = append(cmd, cfg.Server)
		cmd = append(cmd, programName)
		cmd = append(cmd, arg...)
		return exec.Command("ssh", cmd...)
	} else {
		exe, err := os.Readlink("/proc/self/exe")
		if err != nil {
			exe = programName
		}

		return exec.Command(exe, arg...)
	}
}

func switchuser() {
	if cfg.Server == "" && cfg.User != "" {
		if err := Impersonate(cfg.User); err != nil {
			fmt.Fprintln(os.Stderr, "Switch to user", cfg.User, ":", err)
			log.Fatal("Switch to user ", cfg.User, ": ", err)
		}
	}
}
