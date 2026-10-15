//go:build !windows && !linux

package sidecar

import (
	"os/exec"
	"syscall"
)

// configure puts the child in its own process group. Without Linux's
// parent-death signal, a hub killed with SIGKILL can leave it running; the
// next start then fails to bind its ports and says so in the log.
func configure(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}
