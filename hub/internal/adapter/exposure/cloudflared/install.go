package cloudflared

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"time"
)

// Version is the pinned cloudflared release; the checksums come from its
// release notes.
const Version = "2026.10.0"

type asset struct {
	name   string
	sha256 string
	// tgz releases hold the binary as "cloudflared" inside a gzipped tar.
	tgz bool
}

var assets = map[string]asset{
	"windows/amd64": {name: "cloudflared-windows-amd64.exe", sha256: "86aee4017b26625cee8484c113558f48effa4cd47f7aa05fcf425604e5d2b23c"},
	"linux/amd64":   {name: "cloudflared-linux-amd64", sha256: "d33ff2d14475178d2012c2c56beba87389ac5ded27649519f198a7d3134a99db"},
	"linux/arm64":   {name: "cloudflared-linux-arm64", sha256: "e6422b9d4f72d3194bc5a38676f13667c06666523217b842a877d72a80b5ac08"},
	"darwin/amd64":  {name: "cloudflared-darwin-amd64.tgz", sha256: "0560c9ab7281ac3f746055323623ed23bc0405b6dab9400474020cba33a978da", tgz: true},
	"darwin/arm64":  {name: "cloudflared-darwin-arm64.tgz", sha256: "72edfd3eea463aef4d5cb89e2e209cecb048cc756c2b01915de2e0ad7cb39830", tgz: true},
}

const (
	releaseURL      = "https://github.com/cloudflare/cloudflared/releases/download/"
	downloadTimeout = 5 * time.Minute
	maxDownload     = 200 << 20
)

// Installer fetches the pinned cloudflared on first use into Dir.
type Installer struct {
	Dir    string
	Client *http.Client
	// BaseURL overrides the release location, for tests.
	BaseURL string
	// Platform overrides GOOS/GOARCH, for tests.
	Platform string
}

// Ensure returns the path of the pinned cloudflared, downloading and
// verifying it when it is not there yet.
func (in Installer) Ensure(ctx context.Context) (string, error) {
	platform := in.Platform
	if platform == "" {
		platform = runtime.GOOS + "/" + runtime.GOARCH
	}
	a, ok := assets[platform]
	if !ok {
		return "", fmt.Errorf("no cloudflared release for %s", platform)
	}
	name := "cloudflared-" + Version
	if filepath.Ext(a.name) == ".exe" {
		name += ".exe"
	}
	dest := filepath.Join(in.Dir, name)
	stamp := dest + ".sha256"
	if got, err := os.ReadFile(stamp); err == nil && string(got) == a.sha256 {
		if _, err := os.Stat(dest); err == nil {
			return dest, nil
		}
	}
	if err := os.MkdirAll(in.Dir, 0o700); err != nil {
		return "", fmt.Errorf("create sidecar dir: %w", err)
	}
	base := in.BaseURL
	if base == "" {
		base = releaseURL
	}
	ctx, cancel := context.WithTimeout(ctx, downloadTimeout)
	defer cancel()
	archive, err := in.download(ctx, base+Version+"/"+a.name, a.sha256)
	if err != nil {
		return "", err
	}
	defer func() { _ = os.Remove(archive) }()
	tmp := dest + ".part"
	if err := extract(archive, tmp, a.tgz); err != nil {
		_ = os.Remove(tmp)
		return "", err
	}
	if err := os.Rename(tmp, dest); err != nil {
		return "", fmt.Errorf("install cloudflared: %w", err)
	}
	if err := os.WriteFile(stamp, []byte(a.sha256), 0o600); err != nil {
		return "", fmt.Errorf("record cloudflared checksum: %w", err)
	}
	return dest, nil
}

// download saves url to a temporary file and checks its SHA-256.
func (in Installer) download(ctx context.Context, url, want string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
	if err != nil {
		return "", err
	}
	client := in.Client
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("download cloudflared: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download cloudflared: %s", resp.Status)
	}
	f, err := os.CreateTemp(in.Dir, "cloudflared-*.download")
	if err != nil {
		return "", fmt.Errorf("download cloudflared: %w", err)
	}
	h := sha256.New()
	_, err = io.Copy(io.MultiWriter(f, h), io.LimitReader(resp.Body, maxDownload))
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		_ = os.Remove(f.Name())
		return "", fmt.Errorf("download cloudflared: %w", err)
	}
	if got := hex.EncodeToString(h.Sum(nil)); got != want {
		_ = os.Remove(f.Name())
		return "", fmt.Errorf("cloudflared checksum mismatch: got %s, want %s", got, want)
	}
	return f.Name(), nil
}

func extract(archive, dest string, tgz bool) error {
	src, err := os.Open(archive)
	if err != nil {
		return err
	}
	defer func() { _ = src.Close() }()
	var r io.Reader = src
	if tgz {
		zr, err := gzip.NewReader(src)
		if err != nil {
			return fmt.Errorf("read cloudflared archive: %w", err)
		}
		tr := tar.NewReader(zr)
		for {
			h, err := tr.Next()
			if errors.Is(err, io.EOF) {
				return errors.New("cloudflared archive has no cloudflared binary")
			}
			if err != nil {
				return fmt.Errorf("read cloudflared archive: %w", err)
			}
			if filepath.Base(h.Name) == "cloudflared" && h.Typeflag == tar.TypeReg {
				r = tr
				break
			}
		}
	}
	out, err := os.OpenFile(dest, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o700) //nolint:gosec // an executable the hub runs
	if err != nil {
		return err
	}
	_, err = io.Copy(out, io.LimitReader(r, maxDownload))
	if cerr := out.Close(); err == nil {
		err = cerr
	}
	return err
}
