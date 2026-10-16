package hostos

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"syscall"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

const (
	runKey         = `Software\Microsoft\Windows\CurrentVersion\Run`
	runValue       = "Dislackso hub"
	firewallRule   = "Dislackso media"
	esContinuous   = 0x80000000
	esSystemReq    = 0x00000001
	createNoWindow = 0x08000000
)

var setThreadExecutionState = windows.NewLazySystemDLL("kernel32.dll").NewProc("SetThreadExecutionState")

// holdAwake asks Windows not to sleep. The request belongs to the calling
// thread, so a goroutine locked to one thread holds it until released.
func holdAwake(context.Context) (func(), error) {
	errc := make(chan error, 1)
	done := make(chan struct{})
	go func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		if r, _, err := setThreadExecutionState.Call(esContinuous | esSystemReq); r == 0 {
			errc <- fmt.Errorf("SetThreadExecutionState: %w", err)
			return
		}
		errc <- nil
		<-done
		_, _, _ = setThreadExecutionState.Call(esContinuous)
	}()
	if err := <-errc; err != nil {
		return nil, err
	}
	return func() { close(done) }, nil
}

func runAtLoginSupported() bool { return true }

func runAtLogin() (bool, error) {
	k, err := registry.OpenKey(registry.CURRENT_USER, runKey, registry.QUERY_VALUE)
	if err != nil {
		return false, err
	}
	defer func() { _ = k.Close() }()
	_, _, err = k.GetStringValue(runValue)
	if errors.Is(err, registry.ErrNotExist) {
		return false, nil
	}
	return err == nil, err
}

// setRunAtLogin adds or removes the hub from the user's startup programs.
func setRunAtLogin(on bool, l Launch) error {
	k, _, err := registry.CreateKey(registry.CURRENT_USER, runKey, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer func() { _ = k.Close() }()
	if !on {
		if err := k.DeleteValue(runValue); err != nil && !errors.Is(err, registry.ErrNotExist) {
			return err
		}
		return nil
	}
	parts := []string{syscall.EscapeArg(l.Exe)}
	for _, a := range l.Args {
		parts = append(parts, syscall.EscapeArg(a))
	}
	return k.SetStringValue(runValue, strings.Join(parts, " "))
}

func firewallSupported() bool { return true }

// addFirewallRule lets program receive inbound traffic. Windows asks the
// user to confirm (UAC) since the change needs an administrator.
func addFirewallRule(program string) error {
	//nolint:gocritic // netsh quoting: %q would escape the backslashes in the path
	args := fmt.Sprintf(`advfirewall firewall add rule name="%s" dir=in action=allow program="%s" enable=yes`,
		firewallRule, program)
	verb, _ := windows.UTF16PtrFromString("runas")
	file, _ := windows.UTF16PtrFromString("netsh")
	params, err := windows.UTF16PtrFromString(args)
	if err != nil {
		return err
	}
	return windows.ShellExecute(0, verb, file, params, nil, windows.SW_HIDE)
}

func openBrowser(ctx context.Context, url string) error {
	cmd := exec.CommandContext(ctx, "rundll32", "url.dll,FileProtocolHandler", url)
	return cmd.Start()
}
