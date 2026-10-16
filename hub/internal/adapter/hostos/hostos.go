// Package hostos does what hosting needs from the operating system: keep the
// machine awake, start the hub at login, open the firewall for the media
// server, open the browser.
package hostos

import (
	"context"
	"errors"
	"sync"
)

// ErrUnsupported is returned where the operating system offers no way.
var ErrUnsupported = errors.New("not supported on this system")

// Launch is the command that starts the hub at login.
type Launch struct {
	Exe  string
	Args []string
}

// Host is this machine for the host service.
type Host struct {
	Launch Launch
	// Media is the program the firewall rule lets in; empty without voice.
	Media string

	mu      sync.Mutex
	release func()
}

// SetKeepAwake holds the machine awake or lets it sleep; repeated calls are no-ops.
func (h *Host) SetKeepAwake(ctx context.Context, on bool) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if on == (h.release != nil) {
		return nil
	}
	if !on {
		h.release()
		h.release = nil
		return nil
	}
	release, err := holdAwake(context.WithoutCancel(ctx))
	if err != nil {
		return err
	}
	h.release = release
	return nil
}

func (h *Host) RunAtLoginSupported() bool { return runAtLoginSupported() }

func (h *Host) RunAtLogin() (bool, error) { return runAtLogin() }

func (h *Host) SetRunAtLogin(on bool) error { return setRunAtLogin(on, h.Launch) }

func (h *Host) FirewallSupported() bool { return h.Media != "" && firewallSupported() }

func (h *Host) AddFirewallRule() error {
	if h.Media == "" {
		return ErrUnsupported
	}
	return addFirewallRule(h.Media)
}

// OpenBrowser shows url in the default browser.
func OpenBrowser(ctx context.Context, url string) error { return openBrowser(ctx, url) }
