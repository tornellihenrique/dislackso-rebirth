package main

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
)

type installer struct {
	client   *http.Client
	binDir   string
	platform string
	exeExt   string
	force    bool
	log      io.Writer
}

func (in installer) install(ctx context.Context, t tool) error {
	a, err := t.artifactFor(in.platform)
	if err != nil {
		return err
	}
	dest := filepath.Join(in.binDir, t.Name+in.exeExt)
	stampPath := filepath.Join(in.binDir, ".stamps", t.Name)
	stamp := []byte(t.Version + " " + a.SHA256 + "\n")

	if !in.force && upToDate(dest, stampPath, stamp) {
		_, _ = fmt.Fprintf(in.log, "%s %s is up to date\n", t.Name, t.Version)
		return nil
	}

	_, _ = fmt.Fprintf(in.log, "fetching %s %s (%s)\n", t.Name, t.Version, in.platform)
	work, err := os.MkdirTemp("", "fetchtools-*")
	if err != nil {
		return fmt.Errorf("create temp dir: %w", err)
	}
	defer func() { _ = os.RemoveAll(work) }()

	archivePath := filepath.Join(work, "download")
	if err := in.download(ctx, a.URL, a.SHA256, archivePath); err != nil {
		return err
	}

	switch a.Archive {
	case archiveRaw:
		err = copyExecutable(archivePath, dest)
	case archiveZip:
		err = extractZipEntry(archivePath, a.Path, dest)
	case archiveTarGz:
		err = extractTarEntry(archivePath, a.Path, dest)
	case archiveGoSource:
		_, _ = fmt.Fprintf(in.log, "building %s from source (no published binary for %s)\n", t.Name, in.platform)
		err = buildFromSource(ctx, archivePath, filepath.Join(work, "src"), a, dest)
	default:
		err = fmt.Errorf("unknown archive kind %q", a.Archive)
	}
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(stampPath), 0o750); err != nil {
		return fmt.Errorf("create stamp dir: %w", err)
	}
	if err := os.WriteFile(stampPath, stamp, 0o600); err != nil {
		return fmt.Errorf("write stamp: %w", err)
	}
	_, _ = fmt.Fprintf(in.log, "installed %s %s -> %s\n", t.Name, t.Version, dest)
	return nil
}

func upToDate(dest, stampPath string, want []byte) bool {
	if _, err := os.Stat(dest); err != nil {
		return false
	}
	got, err := os.ReadFile(stampPath)
	return err == nil && bytes.Equal(got, want)
}

// download streams url into dest and fails unless its SHA-256 equals wantSHA256.
func (in installer) download(ctx context.Context, url, wantSHA256, dest string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	resp, err := in.client.Do(req)
	if err != nil {
		return fmt.Errorf("download %s: %w", url, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download %s: unexpected status %s", url, resp.Status)
	}

	f, err := os.Create(dest)
	if err != nil {
		return fmt.Errorf("create %s: %w", dest, err)
	}
	h := sha256.New()
	_, copyErr := io.Copy(io.MultiWriter(f, h), resp.Body)
	closeErr := f.Close()
	if err := errors.Join(copyErr, closeErr); err != nil {
		return fmt.Errorf("download %s: %w", url, err)
	}

	if got := hex.EncodeToString(h.Sum(nil)); got != wantSHA256 {
		return fmt.Errorf("checksum mismatch for %s: got %s, want %s", url, got, wantSHA256)
	}
	return nil
}

func copyExecutable(src, dest string) error {
	f, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("open %s: %w", src, err)
	}
	defer func() { _ = f.Close() }()
	return writeExecutable(dest, f)
}

func extractZipEntry(archivePath, entry, dest string) error {
	zr, err := zip.OpenReader(archivePath)
	if err != nil {
		return fmt.Errorf("open zip: %w", err)
	}
	defer func() { _ = zr.Close() }()

	for _, f := range zr.File {
		if f.Name == entry {
			return writeZipFile(f, dest)
		}
	}
	return fmt.Errorf("%s not found in zip", entry)
}

func writeZipFile(f *zip.File, dest string) error {
	rc, err := f.Open()
	if err != nil {
		return fmt.Errorf("open %s in zip: %w", f.Name, err)
	}
	defer func() { _ = rc.Close() }()
	return writeExecutable(dest, rc)
}

func extractTarEntry(archivePath, entry, dest string) error {
	return walkTarGz(archivePath, func(hdr *tar.Header, r io.Reader) (bool, error) {
		if hdr.Typeflag != tar.TypeReg || path.Clean(hdr.Name) != entry {
			return false, nil
		}
		return true, writeExecutable(dest, r)
	}, entry)
}

func buildFromSource(ctx context.Context, archivePath, srcDir string, a artifact, dest string) error {
	err := walkTarGz(archivePath, func(hdr *tar.Header, r io.Reader) (bool, error) {
		return false, extractTarMember(srcDir, hdr, r)
	}, "")
	if err != nil {
		return err
	}

	tmp := dest + ".tmp"
	cmd := exec.CommandContext(ctx, "go", "build", "-trimpath", "-o", tmp, a.Package)
	cmd.Dir = filepath.Join(srcDir, filepath.FromSlash(a.Path))
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("go build %s: %w", a.Package, err)
	}
	if err := os.Rename(tmp, dest); err != nil {
		return fmt.Errorf("install %s: %w", dest, err)
	}
	return nil
}

// walkTarGz calls visit for every member until visit reports done. When
// required is non-empty, reaching the end without done is an error.
func walkTarGz(archivePath string, visit func(*tar.Header, io.Reader) (bool, error), required string) error {
	f, err := os.Open(archivePath)
	if err != nil {
		return fmt.Errorf("open archive: %w", err)
	}
	defer func() { _ = f.Close() }()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return fmt.Errorf("open gzip: %w", err)
	}
	defer func() { _ = gz.Close() }()

	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return fmt.Errorf("read tar: %w", err)
		}
		done, err := visit(hdr, tr)
		if err != nil {
			return err
		}
		if done {
			return nil
		}
	}
	if required != "" {
		return fmt.Errorf("%s not found in archive", required)
	}
	return nil
}

func extractTarMember(root string, hdr *tar.Header, r io.Reader) error {
	if hdr.Typeflag != tar.TypeDir && hdr.Typeflag != tar.TypeReg {
		return nil
	}
	if !filepath.IsLocal(hdr.Name) {
		return fmt.Errorf("refusing archive member outside the target dir: %q", hdr.Name)
	}
	target := filepath.Join(root, filepath.FromSlash(hdr.Name))
	if hdr.Typeflag == tar.TypeDir {
		if err := os.MkdirAll(target, 0o750); err != nil {
			return fmt.Errorf("create %s: %w", target, err)
		}
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o750); err != nil {
		return fmt.Errorf("create %s: %w", filepath.Dir(target), err)
	}
	out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return fmt.Errorf("create %s: %w", target, err)
	}
	_, copyErr := io.Copy(out, r)
	if err := errors.Join(copyErr, out.Close()); err != nil {
		return fmt.Errorf("write %s: %w", target, err)
	}
	return nil
}

// writeExecutable writes through a temp file so an interrupted install never
// leaves a truncated binary at dest.
func writeExecutable(dest string, r io.Reader) error {
	tmp := dest + ".tmp"
	out, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o700) //nolint:gosec // installed tools must be executable
	if err != nil {
		return fmt.Errorf("create %s: %w", tmp, err)
	}
	_, copyErr := io.Copy(out, r)
	if err := errors.Join(copyErr, out.Close()); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("write %s: %w", tmp, err)
	}
	if err := os.Rename(tmp, dest); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("install %s: %w", dest, err)
	}
	return nil
}
