// Package blob stores file contents on the local disk under the data
// directory.
package blob

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/domain"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/ports"
)

// LocalFS keeps each blob at <root>/<k[0:2]>/<k[2:4]>/<k>, so no directory
// grows too large.
type LocalFS struct {
	Root string
}

// validKey accepts lowercase hex only, which also rules out path traversal.
func validKey(key string) bool {
	if len(key) < 8 {
		return false
	}
	for _, r := range key {
		if (r < '0' || r > '9') && (r < 'a' || r > 'f') {
			return false
		}
	}
	return true
}

func (l LocalFS) path(key string) string {
	return filepath.Join(l.Root, key[0:2], key[2:4], key)
}

func (l LocalFS) Put(_ context.Context, key string, r io.Reader) error {
	if !validKey(key) {
		return fmt.Errorf("blob key %q is not lowercase hex", key)
	}
	dst := l.path(key)
	if _, err := os.Stat(dst); err == nil {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o700); err != nil {
		return fmt.Errorf("create blob dir: %w", err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(dst), ".upload-*")
	if err != nil {
		return fmt.Errorf("create blob: %w", err)
	}
	defer func() { _ = os.Remove(tmp.Name()) }()
	if _, err := io.Copy(tmp, r); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write blob: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("sync blob: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close blob: %w", err)
	}
	if err := os.Rename(tmp.Name(), dst); err != nil {
		// Another upload of the same content may have won the race.
		if _, statErr := os.Stat(dst); statErr == nil {
			return nil
		}
		return fmt.Errorf("store blob: %w", err)
	}
	return nil
}

type file struct {
	*os.File
	size int64
}

func (f file) Size() int64 { return f.size }

func (l LocalFS) Open(_ context.Context, key string) (ports.Blob, error) {
	if !validKey(key) {
		return nil, domain.ErrNotFound
	}
	f, err := os.Open(l.path(key))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("open blob: %w", err)
	}
	info, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("stat blob: %w", err)
	}
	return file{File: f, size: info.Size()}, nil
}
