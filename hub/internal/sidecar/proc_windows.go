package sidecar

import (
	"fmt"
	"os"
	"os/exec"
	"sync"
	"unsafe"

	"golang.org/x/sys/windows"
)

// One job object holds every sidecar. Windows closes its handle when the hub
// exits, however it exits, and KILL_ON_JOB_CLOSE then ends the children.
var job struct {
	once   sync.Once
	handle windows.Handle
	err    error
}

func jobHandle() (windows.Handle, error) {
	job.once.Do(func() {
		h, err := windows.CreateJobObject(nil, nil)
		if err != nil {
			job.err = fmt.Errorf("create job object: %w", err)
			return
		}
		info := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{}
		info.BasicLimitInformation.LimitFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
		if _, err := windows.SetInformationJobObject(h, windows.JobObjectExtendedLimitInformation,
			uintptr(unsafe.Pointer(&info)), uint32(unsafe.Sizeof(info))); err != nil {
			_ = windows.CloseHandle(h)
			job.err = fmt.Errorf("configure job object: %w", err)
			return
		}
		job.handle = h
	})
	return job.handle, job.err
}

func configure(*exec.Cmd) {}

func adopt(p *os.Process) error {
	j, err := jobHandle()
	if err != nil {
		return err
	}
	h, err := windows.OpenProcess(windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE, false, uint32(p.Pid)) //nolint:gosec // PIDs fit in 32 bits on Windows
	if err != nil {
		return fmt.Errorf("open process: %w", err)
	}
	defer func() { _ = windows.CloseHandle(h) }()
	if err := windows.AssignProcessToJobObject(j, h); err != nil {
		return fmt.Errorf("assign to job object: %w", err)
	}
	return nil
}

// stop terminates the process: console programs cannot be sent a signal
// from a process without a shared console.
func stop(p *os.Process) { _ = p.Kill() }
