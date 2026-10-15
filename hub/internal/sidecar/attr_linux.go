package sidecar

import (
	"os/exec"
	"syscall"
)

// configure puts the child in its own process group and has the kernel kill
// it if the hub dies.
func configure(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true, Pdeathsig: syscall.SIGKILL}
}
