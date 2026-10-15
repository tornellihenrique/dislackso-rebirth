package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestCompareTrees(t *testing.T) {
	tests := []struct {
		name string
		have map[string]string
		want map[string]string
		diff []string
	}{
		{
			name: "identical",
			have: map[string]string{"a.go": "a", "sub/b.go": "b"},
			want: map[string]string{"a.go": "a", "sub/b.go": "b"},
		},
		{
			name: "changed, missing and extra files",
			have: map[string]string{"a.go": "old", "gone.go": "x"},
			want: map[string]string{"a.go": "new", "sub/new.go": "n"},
			diff: []string{"a.go (changed)", "gone.go (no longer generated)", "sub/new.go (missing)"},
		},
		{
			name: "generated dir absent on disk",
			want: map[string]string{"a.go": "a"},
			diff: []string{"a.go (missing)"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			have := filepath.Join(t.TempDir(), "have")
			want := filepath.Join(t.TempDir(), "want")
			writeTree(t, have, tt.have)
			writeTree(t, want, tt.want)

			got, err := compareTrees(have, want)
			if err != nil {
				t.Fatalf("compareTrees: %v", err)
			}
			if d := cmp.Diff(tt.diff, got); d != "" {
				t.Errorf("diff mismatch (-want +got):\n%s", d)
			}
		})
	}
}

func writeTree(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for rel, content := range files {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
}
