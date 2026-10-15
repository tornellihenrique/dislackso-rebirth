package blob

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/domain"
)

func TestLocalFS(t *testing.T) {
	ctx := t.Context()
	root := t.TempDir()
	store := LocalFS{Root: root}
	key := "0123456789abcdef"

	if err := store.Put(ctx, key, strings.NewReader("first")); err != nil {
		t.Fatalf("Put: %v", err)
	}
	// Keys are content-derived, so a second Put of the same key keeps the first.
	if err := store.Put(ctx, key, strings.NewReader("second")); err != nil {
		t.Fatalf("Put again: %v", err)
	}
	b, err := store.Open(ctx, key)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	data, _ := io.ReadAll(b)
	_ = b.Close()
	if string(data) != "first" || b.Size() != 5 {
		t.Errorf("read %q (size %d)", data, b.Size())
	}
	if _, err := os.Stat(filepath.Join(root, "01", "23", key)); err != nil {
		t.Errorf("blob not sharded by prefix: %v", err)
	}
	if entries, _ := os.ReadDir(filepath.Join(root, "01", "23")); len(entries) != 1 {
		t.Errorf("temporary files left behind: %v", entries)
	}

	if _, err := store.Open(ctx, "fedcba9876543210"); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("missing blob: %v", err)
	}
	for _, bad := range []string{"../../etc/passwd", "ABCDEF0123456789", "abc"} {
		if err := store.Put(ctx, bad, strings.NewReader("x")); err == nil {
			t.Errorf("Put accepted key %q", bad)
		}
		if _, err := store.Open(ctx, bad); !errors.Is(err, domain.ErrNotFound) {
			t.Errorf("Open(%q): %v", bad, err)
		}
	}
}
