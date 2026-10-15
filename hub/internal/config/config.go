// Package config loads the hub's bootstrap settings.
//
// Precedence, lowest first: defaults, config.yaml, DSX_* environment
// variables, command-line flags. Only settings needed before the database is
// open live here; everything else is an instance setting stored in the DB.
package config

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"strings"

	"go.yaml.in/yaml/v3"
)

const (
	envPrefix      = "DSX_"
	configFileName = "config.yaml"
)

// DefaultHTTPAddr keeps the hub private to the host until an exposure
// provider publishes it.
const DefaultHTTPAddr = "127.0.0.1:7300"

type Config struct {
	DataDir string `yaml:"data_dir"`
	HTTP    HTTP   `yaml:"http"`
	Log     Log    `yaml:"log"`
	Web     Web    `yaml:"web"`
	Mail    Mail   `yaml:"mail"`
	Setup   Setup  `yaml:"setup"`
}

type HTTP struct {
	Addr string `yaml:"addr"`
}

type Log struct {
	Level  string `yaml:"level"`
	Format string `yaml:"format"`
}

const (
	LogFormatText = "text"
	LogFormatJSON = "json"
)

type Web struct {
	// Dir serves the web client from disk instead of the embedded build.
	Dir string `yaml:"dir"`
}

type Mail struct {
	// DevLog writes emails to the log while SMTP is disabled, so email flows
	// can be tried without a mail server. Development only.
	DevLog bool `yaml:"dev_log"`
}

type Setup struct {
	// Token fixes the one-time setup token, for headless hosts and tests.
	// Empty means a random token on every start.
	Token string `yaml:"token"`
}

// setting is one key that can be set from config.yaml, the environment and
// flags. The env var and flag names are derived from the key.
type setting struct {
	key   string
	usage string
}

var settings = []setting{
	{key: "data_dir", usage: "directory for the database, files, logs and generated config"},
	{key: "http.addr", usage: "address the hub listens on"},
	{key: "log.level", usage: "log level: debug, info, warn or error"},
	{key: "log.format", usage: "log format: text or json"},
	{key: "web.dir", usage: "serve the web client from this directory instead of the embedded build"},
	{key: "mail.dev_log", usage: "true to log emails instead of sending them while SMTP is off (development)"},
	{key: "setup.token", usage: "use this setup token instead of a random one"},
}

func (s setting) envName() string {
	return envPrefix + strings.ToUpper(strings.NewReplacer(".", "_").Replace(s.key))
}

func (s setting) flagName() string {
	return strings.NewReplacer(".", "-", "_", "-").Replace(s.key)
}

// Source is everything Load reads from the process environment.
type Source struct {
	Args      []string
	LookupEnv func(string) (string, bool)
	// DefaultDataDir returns the per-OS data directory.
	DefaultDataDir func() (string, error)
	// Output receives flag usage and parse errors.
	Output io.Writer
}

// Load resolves the configuration. It returns flag.ErrHelp when -h is passed.
func Load(src Source) (Config, error) {
	fs := flag.NewFlagSet("hub run", flag.ContinueOnError)
	fs.SetOutput(src.Output)
	configPath := fs.String("config", "", "path to config.yaml (default <data-dir>/config.yaml; env "+envPrefix+"CONFIG)")
	flagValues := make(map[string]*string, len(settings))
	for _, s := range settings {
		flagValues[s.key] = fs.String(s.flagName(), "", s.usage+" (env "+s.envName()+")")
	}
	if err := fs.Parse(src.Args); err != nil {
		return Config{}, err
	}
	if fs.NArg() > 0 {
		return Config{}, fmt.Errorf("unexpected arguments: %s", strings.Join(fs.Args(), " "))
	}

	fromFlags := map[string]string{}
	explicitConfig := false
	fs.Visit(func(f *flag.Flag) {
		if f.Name == "config" {
			explicitConfig = true
			return
		}
		for _, s := range settings {
			if s.flagName() == f.Name {
				fromFlags[s.key] = *flagValues[s.key]
			}
		}
	})
	fromEnv := map[string]string{}
	for _, s := range settings {
		if v, ok := src.LookupEnv(s.envName()); ok && v != "" {
			fromEnv[s.key] = v
		}
	}
	if !explicitConfig {
		if v, ok := src.LookupEnv(envPrefix + "CONFIG"); ok && v != "" {
			*configPath = v
			explicitConfig = true
		}
	}

	cfg := defaults()
	switch {
	case fromFlags["data_dir"] != "":
		cfg.DataDir = fromFlags["data_dir"]
	case fromEnv["data_dir"] != "":
		cfg.DataDir = fromEnv["data_dir"]
	default:
		dir, err := src.DefaultDataDir()
		if err != nil {
			return Config{}, fmt.Errorf("resolve data dir: %w", err)
		}
		cfg.DataDir = dir
	}
	if !explicitConfig {
		*configPath = filepath.Join(cfg.DataDir, configFileName)
	}

	if err := applyFile(&cfg, *configPath, explicitConfig); err != nil {
		return Config{}, err
	}
	if err := applyOverrides(&cfg, fromEnv); err != nil {
		return Config{}, fmt.Errorf("apply environment: %w", err)
	}
	if err := applyOverrides(&cfg, fromFlags); err != nil {
		return Config{}, fmt.Errorf("apply flags: %w", err)
	}
	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func defaults() Config {
	return Config{
		HTTP: HTTP{Addr: DefaultHTTPAddr},
		Log:  Log{Level: "info", Format: LogFormatText},
	}
}

// applyFile decodes path over cfg. A missing file is an error only when the
// path was given explicitly.
func applyFile(cfg *Config, path string, required bool) error {
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) && !required {
		return nil
	}
	if err != nil {
		return fmt.Errorf("open config file: %w", err)
	}
	defer func() { _ = f.Close() }()

	dec := yaml.NewDecoder(f)
	dec.KnownFields(true)
	if err := dec.Decode(cfg); err != nil && !errors.Is(err, io.EOF) {
		return fmt.Errorf("parse %s: %w", path, err)
	}
	return nil
}

// applyOverrides decodes dotted key=value pairs as if they were plain YAML
// scalars, so env vars and flags get the same type conversion as the file.
func applyOverrides(cfg *Config, values map[string]string) error {
	if len(values) == 0 {
		return nil
	}
	root := &yaml.Node{Kind: yaml.MappingNode}
	for key, value := range values {
		parent := root
		parts := strings.Split(key, ".")
		for _, part := range parts[:len(parts)-1] {
			parent = childMapping(parent, part)
		}
		parent.Content = append(parent.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Value: parts[len(parts)-1]},
			&yaml.Node{Kind: yaml.ScalarNode, Value: value},
		)
	}
	if err := root.Decode(cfg); err != nil {
		return fmt.Errorf("decode overrides: %w", err)
	}
	return nil
}

func childMapping(parent *yaml.Node, key string) *yaml.Node {
	for i := 0; i+1 < len(parent.Content); i += 2 {
		if parent.Content[i].Value == key {
			return parent.Content[i+1]
		}
	}
	child := &yaml.Node{Kind: yaml.MappingNode}
	parent.Content = append(parent.Content, &yaml.Node{Kind: yaml.ScalarNode, Value: key}, child)
	return child
}

func (c Config) Validate() error {
	var errs []error
	if c.DataDir == "" {
		errs = append(errs, errors.New("data_dir is empty"))
	}
	if _, _, err := net.SplitHostPort(c.HTTP.Addr); err != nil {
		errs = append(errs, fmt.Errorf("http.addr %q: %w", c.HTTP.Addr, err))
	}
	var level slog.Level
	if err := level.UnmarshalText([]byte(c.Log.Level)); err != nil {
		errs = append(errs, fmt.Errorf("log.level %q: %w", c.Log.Level, err))
	}
	if c.Log.Format != LogFormatText && c.Log.Format != LogFormatJSON {
		errs = append(errs, fmt.Errorf("log.format %q: must be %q or %q", c.Log.Format, LogFormatText, LogFormatJSON))
	}
	if err := errors.Join(errs...); err != nil {
		return fmt.Errorf("invalid config: %w", err)
	}
	return nil
}
