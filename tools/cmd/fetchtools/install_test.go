package main

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestArtifactFor(t *testing.T) {
	tl := tool{
		Name:    "demo",
		Version: "1.2.3",
		Platforms: map[string]artifact{
			"linux/amd64": {URL: "https://x/v{version}/demo-{version}.tar.gz", Path: "demo-{version}/demo", Archive: archiveTarGz},
		},
		Source: &artifact{URL: "https://x/src/v{version}.tar.gz", Path: "demo-{version}", Archive: archiveGoSource, Package: "./cmd/demo"},
	}
	noSource := tl
	noSource.Source = nil

	tests := []struct {
		name     string
		tool     tool
		platform string
		wantURL  string
		wantPath string
		wantErr  bool
	}{
		{name: "published binary", tool: tl, platform: "linux/amd64", wantURL: "https://x/v1.2.3/demo-1.2.3.tar.gz", wantPath: "demo-1.2.3/demo"},
		{name: "falls back to source", tool: tl, platform: "darwin/arm64", wantURL: "https://x/src/v1.2.3.tar.gz", wantPath: "demo-1.2.3"},
		{name: "no build and no source", tool: noSource, platform: "darwin/arm64", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a, err := tt.tool.artifactFor(tt.platform)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected an error")
				}
				return
			}
			if err != nil {
				t.Fatalf("artifactFor: %v", err)
			}
			if a.URL != tt.wantURL || a.Path != tt.wantPath {
				t.Errorf("got (%q, %q), want (%q, %q)", a.URL, a.Path, tt.wantURL, tt.wantPath)
			}
		})
	}
}

func TestSelectTools(t *testing.T) {
	m := manifest{Tools: []tool{{Name: "a"}, {Name: "b"}}}
	tests := []struct {
		name    string
		names   []string
		want    []string
		wantErr bool
	}{
		{name: "all by default", want: []string{"a", "b"}},
		{name: "subset in requested order", names: []string{"b", "a"}, want: []string{"b", "a"}},
		{name: "unknown tool", names: []string{"c"}, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := m.selectTools(tt.names)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
			var names []string
			for _, g := range got {
				names = append(names, g.Name)
			}
			if strings.Join(names, ",") != strings.Join(tt.want, ",") {
				t.Errorf("got %v, want %v", names, tt.want)
			}
		})
	}
}

func TestDownloadVerifiesChecksum(t *testing.T) {
	body := []byte("binary contents")
	sum := sha256.Sum256(body)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(body)
	}))
	defer srv.Close()

	in := installer{client: srv.Client()}
	dir := t.TempDir()

	if err := in.download(t.Context(), srv.URL, hex.EncodeToString(sum[:]), filepath.Join(dir, "ok")); err != nil {
		t.Fatalf("download with the right checksum: %v", err)
	}
	if err := in.download(t.Context(), srv.URL, strings.Repeat("0", 64), filepath.Join(dir, "bad")); err == nil {
		t.Fatal("download with a wrong checksum succeeded")
	}
}

func TestExtractZipEntry(t *testing.T) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, content := range map[string]string{"pkg/LICENSE": "license", "pkg/tool.exe": "tool"} {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		_, _ = io.WriteString(w, content)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	archive := filepath.Join(dir, "a.zip")
	writeFile(t, archive, buf.Bytes())

	dest := filepath.Join(dir, "tool.exe")
	if err := extractZipEntry(archive, "pkg/tool.exe", dest); err != nil {
		t.Fatalf("extract: %v", err)
	}
	assertFile(t, dest, "tool")

	if err := extractZipEntry(archive, "pkg/missing", filepath.Join(dir, "missing")); err == nil {
		t.Fatal("extracting a missing entry succeeded")
	}
}

func TestExtractTarEntry(t *testing.T) {
	dir := t.TempDir()
	archive := filepath.Join(dir, "a.tar.gz")
	writeFile(t, archive, tarGz(t, map[string]string{"tool-1/README": "readme", "tool-1/tool": "tool"}))

	dest := filepath.Join(dir, "tool")
	if err := extractTarEntry(archive, "tool-1/tool", dest); err != nil {
		t.Fatalf("extract: %v", err)
	}
	assertFile(t, dest, "tool")

	if err := extractTarEntry(archive, "tool-1/missing", filepath.Join(dir, "missing")); err == nil {
		t.Fatal("extracting a missing entry succeeded")
	}
}

func TestExtractTarMemberRejectsTraversal(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"../escape", "/abs/path", "a/../../escape"} {
		hdr := &tar.Header{Name: name, Typeflag: tar.TypeReg, Size: 1}
		if err := extractTarMember(root, hdr, strings.NewReader("x")); err == nil {
			t.Errorf("member %q was extracted", name)
		}
	}
}

func tarGz(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for name, content := range files {
		hdr := &tar.Header{Name: name, Typeflag: tar.TypeReg, Mode: 0o755, Size: int64(len(content))}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatal(err)
		}
		_, _ = io.WriteString(tw, content)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func writeFile(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
}

func assertFile(t *testing.T, path, want string) {
	t.Helper()
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if string(got) != want {
		t.Errorf("%s = %q, want %q", path, got, want)
	}
}
