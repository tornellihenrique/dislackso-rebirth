package cloudflared

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
)

func sum(b []byte) string {
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

func tgz(t *testing.T, name string, content []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(zw)
	if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0o755, Size: int64(len(content)), Typeflag: tar.TypeReg}); err != nil {
		t.Fatal(err)
	}
	_, _ = tw.Write(content)
	_ = tw.Close()
	_ = zw.Close()
	return buf.Bytes()
}

// release serves files under /<Version>/ and counts downloads.
func release(t *testing.T, files map[string][]byte) (*httptest.Server, *atomic.Int32) {
	t.Helper()
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		body, ok := files[strings.TrimPrefix(r.URL.Path, "/"+Version+"/")]
		if !ok {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write(body)
	}))
	t.Cleanup(srv.Close)
	return srv, &hits
}

func withAssets(t *testing.T, test map[string]asset) {
	t.Helper()
	saved := assets
	assets = test
	t.Cleanup(func() { assets = saved })
}

func TestEnsureDownloadsOnceAndVerifies(t *testing.T) {
	binary := []byte("cloudflared binary")
	packed := tgz(t, "cloudflared", binary)
	withAssets(t, map[string]asset{
		"test/plain": {name: "cloudflared-test", sha256: sum(binary)},
		"test/tgz":   {name: "cloudflared-test.tgz", sha256: sum(packed), tgz: true},
		"test/bad":   {name: "cloudflared-bad", sha256: sum([]byte("something else"))},
	})
	srv, hits := release(t, map[string][]byte{
		"cloudflared-test":     binary,
		"cloudflared-test.tgz": packed,
		"cloudflared-bad":      binary,
	})

	for _, platform := range []string{"test/plain", "test/tgz"} {
		t.Run(platform, func(t *testing.T) {
			hits.Store(0)
			in := Installer{Dir: t.TempDir(), Client: srv.Client(), BaseURL: srv.URL + "/", Platform: platform}
			path, err := in.Ensure(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			if got, _ := os.ReadFile(path); !bytes.Equal(got, binary) {
				t.Errorf("installed %q, want the binary", got)
			}
			if _, err := in.Ensure(t.Context()); err != nil || hits.Load() != 1 {
				t.Errorf("second Ensure: err %v, downloads %d, want the installed copy", err, hits.Load())
			}
		})
	}

	t.Run("checksum mismatch", func(t *testing.T) {
		dir := t.TempDir()
		in := Installer{Dir: dir, Client: srv.Client(), BaseURL: srv.URL + "/", Platform: "test/bad"}
		if _, err := in.Ensure(t.Context()); err == nil || !strings.Contains(err.Error(), "checksum") {
			t.Fatalf("err = %v, want a checksum mismatch", err)
		}
		if entries, _ := os.ReadDir(dir); len(entries) != 0 {
			t.Errorf("left files behind: %v", entries)
		}
	})
}

func TestEnsureRefusesUnknownPlatforms(t *testing.T) {
	if _, err := (Installer{Dir: t.TempDir(), Platform: "plan9/mips"}).Ensure(t.Context()); err == nil {
		t.Error("no error for a platform without a release")
	}
}
