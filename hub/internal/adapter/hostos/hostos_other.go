//go:build !windows && !darwin

package hostos

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// holdAwake keeps systemd from sleeping while the hub runs; hosts without
// systemd stay as they are.
func holdAwake(ctx context.Context) (func(), error) {
	cmd := exec.CommandContext(ctx, "systemd-inhibit", "--what=idle:sleep", "--who=Dislackso", //nolint:gosec // only our own PID varies
		"--why=Hosting a Dislackso instance", "--mode=block",
		"sh", "-c", "while kill -0 "+strconv.Itoa(os.Getpid())+" 2>/dev/null; do sleep 30; done")
	if err := cmd.Start(); err != nil {
		return nil, ErrUnsupported
	}
	return func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	}, nil
}

func autostartPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "autostart", "dislackso-hub.desktop"), nil
}

func runAtLoginSupported() bool { return true }

func runAtLogin() (bool, error) {
	p, err := autostartPath()
	if err != nil {
		return false, err
	}
	_, err = os.Stat(p)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	return err == nil, err
}

// setRunAtLogin writes or removes an XDG autostart entry for desktop sessions.
func setRunAtLogin(on bool, l Launch) error {
	p, err := autostartPath()
	if err != nil {
		return err
	}
	if !on {
		if err := os.Remove(p); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		return nil
	}
	quoted := make([]string, 0, len(l.Args)+1)
	for _, a := range append([]string{l.Exe}, l.Args...) {
		quoted = append(quoted, `"`+strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(a)+`"`)
	}
	entry := "[Desktop Entry]\nType=Application\nName=Dislackso hub\nExec=" + strings.Join(quoted, " ") +
		"\nX-GNOME-Autostart-enabled=true\n"
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return err
	}
	return os.WriteFile(p, []byte(entry), 0o600)
}

func firewallSupported() bool { return false }

func addFirewallRule(string) error { return ErrUnsupported }

func openBrowser(ctx context.Context, url string) error {
	return exec.CommandContext(ctx, "xdg-open", url).Start()
}
