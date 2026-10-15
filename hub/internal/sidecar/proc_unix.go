//go:build !windows

package sidecar

import (
	"os"
	"syscall"
)

func adopt(*os.Process) error { return nil }

// stop asks the process group to exit; waitOrKill kills it if it does not.
func stop(p *os.Process) { _ = syscall.Kill(-p.Pid, syscall.SIGTERM) }
