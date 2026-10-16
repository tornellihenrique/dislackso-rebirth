// Package sidecar runs the helper processes the hub depends on (the media
// server, tunnels): it starts them, waits until they are healthy, restarts
// them with backoff when they exit and makes sure none outlives the hub.
package sidecar

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"
)

type State string

const (
	StateStarting   State = "starting"
	StateRunning    State = "running"
	StateRestarting State = "restarting"
	StateStopped    State = "stopped"
)

const (
	healthInterval = 250 * time.Millisecond
	// healthTimeout restarts a process that never becomes healthy.
	healthTimeout = 30 * time.Second
	stopTimeout   = 5 * time.Second
	minBackoff    = time.Second
	maxBackoff    = 30 * time.Second
	// stableAfter is the uptime after which a crash restarts at once again.
	stableAfter = time.Minute
	// maxLogBytes rotates the log at start once it grows past this size.
	maxLogBytes = 10 << 20
)

type Spec struct {
	Name string
	Path string
	Args []string
	// Env is added to the hub's environment.
	Env []string
	// LogPath receives the process's output.
	LogPath string
	// Health reports whether the process serves; it is polled after each start.
	Health func(ctx context.Context) error
	// OnReady runs each time the process becomes healthy; restarts counts
	// the earlier exits.
	OnReady func(ctx context.Context, restarts int)
	// OnOutput receives each line the process writes, besides the log file.
	OnOutput func(line string)
}

type Status struct {
	State     State
	PID       int
	Restarts  int
	LastError string
	Since     time.Time
}

type Supervisor struct {
	spec   Spec
	logger *slog.Logger

	mu     sync.Mutex
	status Status
}

func New(spec Spec, logger *slog.Logger) *Supervisor {
	return &Supervisor{
		spec:   spec,
		logger: logger.With(slog.String("sidecar", spec.Name)),
		status: Status{State: StateStopped, Since: time.Now()},
	}
}

func (s *Supervisor) Status() Status {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.status
}

// Ready reports whether the process is running and healthy.
func (s *Supervisor) Ready() bool { return s.Status().State == StateRunning }

func (s *Supervisor) set(fn func(*Status)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	prev := s.status.State
	fn(&s.status)
	if s.status.State != prev {
		s.status.Since = time.Now()
	}
}

// Run keeps the process running until ctx is done, then stops it.
func (s *Supervisor) Run(ctx context.Context) error {
	if _, err := os.Stat(s.spec.Path); err != nil {
		return fmt.Errorf("%s binary: %w", s.spec.Name, err)
	}
	backoff := minBackoff
	for {
		started := time.Now()
		err := s.runOnce(ctx)
		if ctx.Err() != nil {
			s.set(func(st *Status) { st.State, st.PID = StateStopped, 0 })
			return nil //nolint:nilerr // stopping is how Run ends, whatever the last run returned
		}
		if time.Since(started) >= stableAfter {
			backoff = minBackoff
		}
		s.set(func(st *Status) {
			st.State, st.PID = StateRestarting, 0
			st.Restarts++
			st.LastError = err.Error()
		})
		s.logger.WarnContext(ctx, "sidecar exited; restarting", slog.Any("error", err), slog.Duration("in", backoff))
		select {
		case <-ctx.Done():
			s.set(func(st *Status) { st.State = StateStopped })
			return nil
		case <-time.After(backoff):
		}
		backoff = min(backoff*2, maxBackoff)
	}
}

// runOnce starts the process and returns when it exits or ctx is done.
func (s *Supervisor) runOnce(ctx context.Context) error {
	logFile, err := openLog(s.spec.LogPath)
	if err != nil {
		return err
	}
	defer func() { _ = logFile.Close() }()

	cmd := exec.CommandContext(ctx, s.spec.Path, s.spec.Args...) //nolint:gosec // the path and arguments come from the hub's own configuration
	// Ending ctx asks the process to stop, then kills it if it lingers.
	cmd.Cancel = func() error {
		stop(cmd.Process)
		return nil
	}
	cmd.WaitDelay = stopTimeout
	cmd.Env = append(os.Environ(), s.spec.Env...)
	var out io.Writer = logFile
	if s.spec.OnOutput != nil {
		tap := &lineTap{fn: s.spec.OnOutput}
		defer tap.flush()
		out = io.MultiWriter(logFile, tap)
	}
	cmd.Stdout, cmd.Stderr = out, out
	configure(cmd)
	s.set(func(st *Status) { st.State = StateStarting })
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start %s: %w", s.spec.Name, err)
	}
	if err := adopt(cmd.Process); err != nil {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		return fmt.Errorf("tie %s to the hub: %w", s.spec.Name, err)
	}
	pid := cmd.Process.Pid
	s.set(func(st *Status) { st.PID = pid })
	s.logger.InfoContext(ctx, "sidecar started", slog.Int("pid", pid))

	exited := make(chan error, 1)
	go func() { exited <- cmd.Wait() }()
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	unhealthy := make(chan error, 1)
	go s.watchHealth(runCtx, unhealthy)

	select {
	case err := <-exited:
		if ctx.Err() != nil {
			s.logger.InfoContext(ctx, "sidecar stopped")
			return ctx.Err()
		}
		return exitError(err)
	case err := <-unhealthy:
		stop(cmd.Process)
		waitOrKill(cmd.Process, exited)
		return err
	}
}

// watchHealth polls until the process is healthy, then runs OnReady.
func (s *Supervisor) watchHealth(ctx context.Context, unhealthy chan<- error) {
	deadline := time.Now().Add(healthTimeout)
	tick := time.NewTicker(healthInterval)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
		}
		err := s.spec.Health(ctx)
		if err == nil {
			restarts := 0
			s.set(func(st *Status) {
				st.State, st.LastError = StateRunning, ""
				restarts = st.Restarts
			})
			s.logger.InfoContext(ctx, "sidecar ready")
			if s.spec.OnReady != nil {
				s.spec.OnReady(ctx, restarts)
			}
			return
		}
		if time.Now().After(deadline) {
			unhealthy <- fmt.Errorf("%s not healthy after %s: %w", s.spec.Name, healthTimeout, err)
			return
		}
	}
}

func waitOrKill(p *os.Process, exited <-chan error) {
	select {
	case <-exited:
	case <-time.After(stopTimeout):
		_ = p.Kill()
		<-exited
	}
}

func exitError(err error) error {
	if err == nil {
		return errors.New("exited with status 0")
	}
	return err
}

func openLog(path string) (*os.File, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("create log dir: %w", err)
	}
	if info, err := os.Stat(path); err == nil && info.Size() > maxLogBytes {
		_ = os.Rename(path, path+".1")
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open sidecar log: %w", err)
	}
	return f, nil
}

// lineTap hands complete lines to fn; exec writes to it from one goroutine
// per stream, so it locks.
type lineTap struct {
	mu  sync.Mutex
	buf []byte
	fn  func(string)
}

func (t *lineTap) Write(p []byte) (int, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.buf = append(t.buf, p...)
	for {
		i := bytes.IndexByte(t.buf, '\n')
		if i < 0 {
			break
		}
		t.fn(string(bytes.TrimRight(t.buf[:i], "\r")))
		t.buf = t.buf[i+1:]
	}
	return len(p), nil
}

func (t *lineTap) flush() {
	t.mu.Lock()
	defer t.mu.Unlock()
	if len(t.buf) > 0 {
		t.fn(string(t.buf))
		t.buf = nil
	}
}
