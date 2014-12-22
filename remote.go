package main

import (
	"os"
	"os/exec"
)

func remotecommand(arg ...string) *exec.Cmd {
	os.Setenv("SSH_CLIENT", "")
	os.Setenv("SSH_CONNECTION", "")

	if cfg.Server != "" {
		cmd := []string{"-q", "-C", "-oBatchMode=yes", "-oStrictHostKeyChecking=no", "-oUserKnownHostsFile=/dev/null"}
		if cfg.User != "" {
			cmd = append(cmd, "-l", cfg.User)
		}
		cmd = append(cmd, cfg.Server)
		cmd = append(cmd, programName)
		cmd = append(cmd, arg...)
		return exec.Command("ssh", cmd...)
	} else {

		if exe, err := os.Readlink("/proc/self/exe"); err == nil {
			return exec.Command(exe, arg...)
		} else {
			return exec.Command(programName, arg...)
		}
	}
}
