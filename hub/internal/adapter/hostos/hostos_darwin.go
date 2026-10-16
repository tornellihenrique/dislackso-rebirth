package hostos

import (
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
)

const agentLabel = "com.dislackso.hub"

// holdAwake runs caffeinate tied to the hub's process, so the hold ends with
// the hub even if it crashes.
func holdAwake(ctx context.Context) (func(), error) {
	cmd := exec.CommandContext(ctx, "caffeinate", "-i", "-w", strconv.Itoa(os.Getpid())) //nolint:gosec // only our own PID varies
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	}, nil
}

func agentPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "Library", "LaunchAgents", agentLabel+".plist"), nil
}

func runAtLoginSupported() bool { return true }

func runAtLogin() (bool, error) {
	p, err := agentPath()
	if err != nil {
		return false, err
	}
	_, err = os.Stat(p)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	return err == nil, err
}

// setRunAtLogin writes or removes a LaunchAgent that starts the hub at login.
func setRunAtLogin(on bool, l Launch) error {
	p, err := agentPath()
	if err != nil {
		return err
	}
	if !on {
		if err := os.Remove(p); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		return nil
	}
	var args bytes.Buffer
	for _, a := range append([]string{l.Exe}, l.Args...) {
		args.WriteString("<string>")
		_ = xml.EscapeText(&args, []byte(a))
		args.WriteString("</string>")
	}
	plist := `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict>
<key>Label</key><string>` + agentLabel + `</string>
<key>ProgramArguments</key><array>` + args.String() + `</array>
<key>RunAtLoad</key><true/>
</dict></plist>
`
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil { //nolint:gosec // the user's LaunchAgents directory
		return err
	}
	return os.WriteFile(p, []byte(plist), 0o600)
}

func firewallSupported() bool { return false }

func addFirewallRule(string) error { return ErrUnsupported }

func openBrowser(ctx context.Context, url string) error {
	return exec.CommandContext(ctx, "open", url).Start()
}
