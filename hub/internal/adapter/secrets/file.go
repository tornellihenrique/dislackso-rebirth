// Package secrets keeps credentials in <data-dir>/secrets.yaml (mode 0600),
// outside the database so backups and replicas never carry them.
package secrets

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"sync"

	"go.yaml.in/yaml/v3"
)

type File struct {
	path string

	mu     sync.Mutex
	values map[string]string
}

// OpenFile loads path, or starts empty when it does not exist yet.
func OpenFile(path string) (*File, error) {
	f := &File{path: path, values: map[string]string{}}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return f, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	if err := yaml.Unmarshal(data, &f.values); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	if f.values == nil {
		f.values = map[string]string{}
	}
	return f, nil
}

func (f *File) Secret(_ context.Context, key string) (string, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	v, ok := f.values[key]
	return v, ok, nil
}

// SetSecret stores value under key; an empty value removes the key.
func (f *File) SetSecret(_ context.Context, key, value string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	next := maps.Clone(f.values)
	if value == "" {
		delete(next, key)
	} else {
		next[key] = value
	}
	if err := f.write(next); err != nil {
		return err
	}
	f.values = next
	return nil
}

// write replaces the file atomically, so a crash never leaves it truncated.
func (f *File) write(values map[string]string) error {
	data, err := yaml.Marshal(values)
	if err != nil {
		return fmt.Errorf("encode secrets: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(f.path), 0o700); err != nil {
		return fmt.Errorf("create %s: %w", filepath.Dir(f.path), err)
	}
	tmp := f.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return fmt.Errorf("write %s: %w", tmp, err)
	}
	if err := os.Rename(tmp, f.path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("replace %s: %w", f.path, err)
	}
	return nil
}
