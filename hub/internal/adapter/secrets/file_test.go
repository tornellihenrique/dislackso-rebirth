package secrets

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestFile(t *testing.T) {
	ctx := t.Context()
	path := filepath.Join(t.TempDir(), "nested", "secrets.yaml")

	f, err := OpenFile(path)
	if err != nil {
		t.Fatalf("OpenFile on a missing file: %v", err)
	}
	if _, ok, _ := f.Secret(ctx, "smtp.password"); ok {
		t.Error("empty store returned a secret")
	}

	if err := f.SetSecret(ctx, "smtp.password", "s3cret: with yaml chars #"); err != nil {
		t.Fatal(err)
	}
	if err := f.SetSecret(ctx, "other", "x"); err != nil {
		t.Fatal(err)
	}
	if err := f.SetSecret(ctx, "other", ""); err != nil {
		t.Fatal(err)
	}

	reopened, err := OpenFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if v, ok, _ := reopened.Secret(ctx, "smtp.password"); !ok || v != "s3cret: with yaml chars #" {
		t.Errorf("after reopen: %q, %v", v, ok)
	}
	if _, ok, _ := reopened.Secret(ctx, "other"); ok {
		t.Error("an empty value should remove the key")
	}

	if runtime.GOOS != "windows" {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if perm := info.Mode().Perm(); perm != 0o600 {
			t.Errorf("mode = %o, want 600", perm)
		}
	}
}
