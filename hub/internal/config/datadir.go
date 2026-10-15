package config

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"

	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/product"
)

// DefaultDataDir returns the per-OS data directory for the running process.
func DefaultDataDir() (string, error) {
	return dataDirFor(runtime.GOOS, os.LookupEnv, os.UserHomeDir)
}

// dataDirFor resolves:
//
//	windows  %APPDATA%\<Name>\hub
//	darwin   ~/Library/Application Support/<Name>/hub
//	other    $XDG_DATA_HOME/<slug>/hub, else ~/.local/share/<slug>/hub
func dataDirFor(goos string, lookupEnv func(string) (string, bool), homeDir func() (string, error)) (string, error) {
	switch goos {
	case "windows":
		appData, ok := lookupEnv("APPDATA")
		if !ok || appData == "" {
			return "", errors.New("%APPDATA% is not set")
		}
		return filepath.Join(appData, product.Name, "hub"), nil
	case "darwin":
		home, err := homeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, "Library", "Application Support", product.Name, "hub"), nil
	default:
		if xdg, ok := lookupEnv("XDG_DATA_HOME"); ok && filepath.IsAbs(xdg) {
			return filepath.Join(xdg, product.Slug, "hub"), nil
		}
		home, err := homeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, ".local", "share", product.Slug, "hub"), nil
	}
}
