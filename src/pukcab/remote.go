package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"strconv"
)

func sshcopyid() error {
	cmd := []string{"-i", "-oStrictHostKeyChecking=no", "-oUserKnownHostsFile=/dev/null"}
	if cfg.Port > 0 {
		cmd = append(cmd, "-p", strconv.Itoa(cfg.Port))
	}
	if cfg.User != "" {
		cmd = append(cmd, cfg.User+"@"+cfg.Server)
	} else {
		cmd = append(cmd, cfg.Server)
	}
	copyid := exec.Command("ssh-copy-id", cmd...)

	copyid.Stderr = os.Stderr
	copyid.Stdout = os.Stdout
	copyid.Stdin = os.Stdin

	if err := copyid.Start(); err != nil {
		return err
	}
	if err := copyid.Wait(); err != nil {
		return err
	}

	return nil
}

func ssh(arg ...string) *exec.Cmd {
	cmd := []string{"-C", "-oLogLevel=ERROR", "-oBatchMode=yes", "-oStrictHostKeyChecking=no", "-oUserKnownHostsFile=/dev/null"}
	if cfg.User != "" {
		cmd = append(cmd, "-l", cfg.User)
	}
	if cfg.Port > 0 {
		cmd = append(cmd, "-p", strconv.Itoa(cfg.Port))
	}
	cmd = append(cmd, cfg.Server)
	cmd = append(cmd, arg...)
	return exec.Command("ssh", cmd...)
}

func remotecommand(arg ...string) (rcmd *exec.Cmd) {
	os.Setenv("SSH_CLIENT", "")
	os.Setenv("SSH_CONNECTION", "")

	if cfg.Server != "" {
		cmd := []string{cfg.Command}
		cmd = append(cmd, arg[0])
		if protocol > 0 {
			cmd = append(cmd, "-protocol", strconv.Itoa(protocol))
		} else {
			cmd = append(cmd, "-protocol", strconv.Itoa(protocolVersion))
		}
		if timeout != defaultTimeout {
			cmd = append(cmd, "-timeout", strconv.Itoa(timeout))
		}
		cmd = append(cmd, arg[1:]...)
		rcmd = ssh(cmd...)
	} else {
		cmd := []string{arg[0]}
		if timeout != defaultTimeout {
			cmd = append(cmd, "-timeout", strconv.Itoa(timeout))
		}
		cmd = append(cmd, arg[1:]...)
		rcmd = exec.Command(programFile, cmd...)
	}
	rcmd.Stderr = NewLogStream(failure)
	return rcmd
}

func switchuser() {
	if cfg.Server == "" && cfg.User != "" {
		if err := Impersonate(cfg.User); err != nil {
			fmt.Fprintln(os.Stderr, "Switch to user", cfg.User, ":", err)
			log.Fatal("Switch to user ", cfg.User, ": ", err)
		}
	}
}
