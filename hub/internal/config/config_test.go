package config

import (
	"errors"
	"flag"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestLoad(t *testing.T) {
	dataDir := t.TempDir()
	writeConfig(t, filepath.Join(dataDir, "config.yaml"), "http:\n  addr: 0.0.0.0:8000\nlog:\n  level: debug\n")

	otherDir := t.TempDir()
	explicit := filepath.Join(otherDir, "custom.yaml")
	writeConfig(t, explicit, "data_dir: "+filepath.ToSlash(otherDir)+"\nlog:\n  format: json\n")

	base := func(mut func(*Config)) Config {
		c := Config{
			DataDir: dataDir,
			HTTP:    HTTP{Addr: "0.0.0.0:8000"},
			Log:     Log{Level: "debug", Format: LogFormatText},
			LiveKit: defaultLiveKit,
		}
		if mut != nil {
			mut(&c)
		}
		return c
	}

	tests := []struct {
		name string
		args []string
		env  map[string]string
		want Config
	}{
		{
			name: "file in data dir over defaults",
			want: base(nil),
		},
		{
			name: "env over file",
			env:  map[string]string{"DSX_HTTP_ADDR": "127.0.0.1:9000", "DSX_LOG_FORMAT": "json"},
			want: base(func(c *Config) { c.HTTP.Addr = "127.0.0.1:9000"; c.Log.Format = LogFormatJSON }),
		},
		{
			name: "flags over env",
			args: []string{"--http-addr", "127.0.0.1:9100", "--web-dir", "/srv/web"},
			env:  map[string]string{"DSX_HTTP_ADDR": "127.0.0.1:9000"},
			want: base(func(c *Config) { c.HTTP.Addr = "127.0.0.1:9100"; c.Web.Dir = "/srv/web" }),
		},
		{
			name: "booleans and strings from flags and env",
			args: []string{"--mail-dev-log=true"},
			env:  map[string]string{"DSX_SETUP_TOKEN": "fixed"},
			want: base(func(c *Config) { c.Mail.DevLog = true; c.Setup.Token = "fixed" }),
		},
		{
			name: "livekit ports and switches",
			args: []string{"--livekit-udp-port", "17882", "--livekit-external-ip=false"},
			env:  map[string]string{"DSX_LIVEKIT_BINARY": "/opt/livekit-server"},
			want: base(func(c *Config) {
				c.LiveKit.UDPPort, c.LiveKit.ExternalIP, c.LiveKit.Binary = 17882, false, "/opt/livekit-server"
			}),
		},
		{
			name: "empty env var is ignored",
			env:  map[string]string{"DSX_HTTP_ADDR": ""},
			want: base(nil),
		},
		{
			name: "explicit config file via env",
			env:  map[string]string{"DSX_CONFIG": explicit},
			want: Config{
				DataDir: filepath.ToSlash(otherDir),
				HTTP:    HTTP{Addr: DefaultHTTPAddr},
				Log:     Log{Level: "info", Format: LogFormatJSON},
				LiveKit: defaultLiveKit,
			},
		},
		{
			name: "missing default config file falls back to defaults",
			args: []string{"--data-dir", otherDir},
			want: Config{
				DataDir: otherDir, HTTP: HTTP{Addr: DefaultHTTPAddr}, Log: Log{Level: "info", Format: LogFormatText},
				LiveKit: defaultLiveKit,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Load(source(tt.args, tt.env, dataDir))
			if err != nil {
				t.Fatalf("Load: %v", err)
			}
			if d := cmp.Diff(tt.want, got); d != "" {
				t.Errorf("config mismatch (-want +got):\n%s", d)
			}
		})
	}
}

var defaultLiveKit = LiveKit{Port: 7880, UDPPort: 7882, TCPPort: 7881, ExternalIP: true}

func TestLoadErrors(t *testing.T) {
	dataDir := t.TempDir()
	typo := filepath.Join(dataDir, "typo.yaml")
	writeConfig(t, typo, "htp:\n  addr: x\n")

	tests := []struct {
		name string
		args []string
		env  map[string]string
	}{
		{name: "unknown key in file", args: []string{"--config", typo}},
		{name: "explicit config file missing", args: []string{"--config", filepath.Join(dataDir, "nope.yaml")}},
		{name: "invalid log level", env: map[string]string{"DSX_LOG_LEVEL": "loud"}},
		{name: "invalid log format", args: []string{"--log-format", "xml"}},
		{name: "invalid listen address", args: []string{"--http-addr", "7300"}},
		{name: "livekit port out of range", args: []string{"--livekit-port", "70000"}},
		{name: "livekit ports collide", args: []string{"--livekit-tcp-port", "7882"}},
		{name: "unknown flag", args: []string{"--nope"}},
		{name: "positional argument", args: []string{"extra"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := Load(source(tt.args, tt.env, dataDir)); err == nil {
				t.Fatal("expected an error")
			}
		})
	}
}

func TestLoadHelp(t *testing.T) {
	_, err := Load(source([]string{"-h"}, nil, t.TempDir()))
	if !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("err = %v, want flag.ErrHelp", err)
	}
}

func TestDataDirFor(t *testing.T) {
	homeDir := filepath.FromSlash("/home/u")
	appData := filepath.FromSlash("C:/Users/u/AppData/Roaming")
	xdg := t.TempDir()
	env := func(vars map[string]string) func(string) (string, bool) {
		return func(k string) (string, bool) { v, ok := vars[k]; return v, ok }
	}

	tests := []struct {
		name    string
		goos    string
		env     map[string]string
		noHome  bool
		want    string
		wantErr bool
	}{
		{name: "windows", goos: "windows", env: map[string]string{"APPDATA": appData}, want: filepath.Join(appData, "Dislackso", "hub")},
		{name: "windows without APPDATA", goos: "windows", wantErr: true},
		{name: "darwin", goos: "darwin", want: filepath.Join(homeDir, "Library", "Application Support", "Dislackso", "hub")},
		{name: "linux xdg", goos: "linux", env: map[string]string{"XDG_DATA_HOME": xdg}, want: filepath.Join(xdg, "dislackso", "hub")},
		{name: "linux default", goos: "linux", want: filepath.Join(homeDir, ".local", "share", "dislackso", "hub")},
		{name: "darwin without home", goos: "darwin", noHome: true, wantErr: true},
		{name: "linux without home", goos: "linux", noHome: true, wantErr: true},
		{name: "linux ignores relative xdg", goos: "linux", env: map[string]string{"XDG_DATA_HOME": "rel"}, want: filepath.Join(homeDir, ".local", "share", "dislackso", "hub")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			home := func() (string, error) {
				if tt.noHome {
					return "", errors.New("no home directory")
				}
				return homeDir, nil
			}
			got, err := dataDirFor(tt.goos, env(tt.env), home)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func source(args []string, env map[string]string, dataDir string) Source {
	return Source{
		Args:           args,
		LookupEnv:      func(k string) (string, bool) { v, ok := env[k]; return v, ok },
		DefaultDataDir: func() (string, error) { return dataDir, nil },
		Output:         io.Discard,
	}
}

func writeConfig(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}
