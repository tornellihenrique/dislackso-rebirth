// Command pack writes release archives: a .zip (Windows) or .tar.gz holding
// the given files in one top-level folder, executables marked as such.
package main

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
)

func main() {
	out := flag.String("o", "", "archive to write (.zip or .tar.gz)")
	prefix := flag.String("prefix", "", "folder the files go in")
	flag.Parse()
	if err := pack(*out, *prefix, flag.Args()); err != nil {
		fmt.Fprintln(os.Stderr, "pack:", err)
		os.Exit(1)
	}
}

// Archive entries carry a fixed time, so the same inputs give the same archive.
var stamp = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

func pack(out, prefix string, files []string) error {
	if out == "" || len(files) == 0 {
		return errors.New("usage: pack -o out.zip|out.tar.gz [-prefix dir] file [file]")
	}
	if err := os.MkdirAll(filepath.Dir(out), 0o750); err != nil {
		return err
	}
	f, err := os.Create(out)
	if err != nil {
		return err
	}
	switch {
	case strings.HasSuffix(out, ".zip"):
		err = writeZip(f, prefix, files)
	case strings.HasSuffix(out, ".tar.gz"):
		err = writeTarGz(f, prefix, files)
	default:
		err = fmt.Errorf("unknown archive type: %s", out)
	}
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		_ = os.Remove(out)
	}
	return err
}

func mode(name string) os.FileMode {
	if strings.HasSuffix(name, ".txt") || strings.HasSuffix(name, ".md") {
		return 0o644
	}
	return 0o755
}

func writeZip(w io.Writer, prefix string, files []string) error {
	zw := zip.NewWriter(w)
	for _, file := range files {
		name := path.Join(prefix, filepath.Base(file))
		h := &zip.FileHeader{Name: name, Method: zip.Deflate, Modified: stamp}
		h.SetMode(mode(name))
		dst, err := zw.CreateHeader(h)
		if err != nil {
			return err
		}
		if err := copyFile(dst, file); err != nil {
			return err
		}
	}
	return zw.Close()
}

func writeTarGz(w io.Writer, prefix string, files []string) error {
	gz := gzip.NewWriter(w)
	tw := tar.NewWriter(gz)
	for _, file := range files {
		info, err := os.Stat(file)
		if err != nil {
			return err
		}
		name := path.Join(prefix, filepath.Base(file))
		h := &tar.Header{Name: name, Mode: int64(mode(name)), Size: info.Size(), ModTime: stamp, Typeflag: tar.TypeReg}
		if err := tw.WriteHeader(h); err != nil {
			return err
		}
		if err := copyFile(tw, file); err != nil {
			return err
		}
	}
	if err := tw.Close(); err != nil {
		return err
	}
	return gz.Close()
}

func copyFile(dst io.Writer, file string) error {
	src, err := os.Open(file)
	if err != nil {
		return err
	}
	defer func() { _ = src.Close() }()
	_, err = io.Copy(dst, src)
	return err
}
