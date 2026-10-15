// Command gencheck verifies that generated code on disk matches what the
// generator produces now. It runs the generator into a scratch directory and
// compares the result with the working tree, so it works on uncommitted
// changes as well as in CI.
//
// Usage: gencheck -root <repo> -dir <workdir> -path <generated dir>... -- <generator> [args...]
//
// The token {out} in generator arguments is replaced with the scratch
// directory's counterpart of -dir.
package main

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"slices"
	"strings"
)

type pathList []string

func (p *pathList) String() string { return strings.Join(*p, ",") }

func (p *pathList) Set(v string) error {
	*p = append(*p, v)
	return nil
}

func main() {
	root := flag.String("root", ".", "repository root")
	workdir := flag.String("dir", ".", "directory to run the generator in, relative to -root")
	var paths pathList
	flag.Var(&paths, "path", "generated directory to compare, relative to -root (repeatable)")
	flag.Parse()

	os.Exit(check(*root, *workdir, paths, flag.Args()))
}

func check(root, workdir string, paths, generator []string) int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	stale, err := run(ctx, root, workdir, paths, generator)
	if err != nil {
		fmt.Fprintln(os.Stderr, "gencheck:", err)
		return 2
	}
	if len(stale) > 0 {
		fmt.Fprintln(os.Stderr, "generated code is stale; run `task gen` and commit the result:")
		for _, s := range stale {
			fmt.Fprintln(os.Stderr, "  "+s)
		}
		return 1
	}
	fmt.Println("generated code is up to date")
	return 0
}

func run(ctx context.Context, root, workdir string, paths, generator []string) ([]string, error) {
	if len(paths) == 0 || len(generator) == 0 {
		return nil, errors.New("need at least one -path and a generator command after --")
	}
	scratch, err := os.MkdirTemp("", "gencheck-*")
	if err != nil {
		return nil, fmt.Errorf("create scratch dir: %w", err)
	}
	defer func() { _ = os.RemoveAll(scratch) }()

	out := filepath.Join(scratch, filepath.FromSlash(workdir))
	args := make([]string, len(generator)-1)
	for i, a := range generator[1:] {
		args[i] = strings.ReplaceAll(a, "{out}", out)
	}
	cmd := exec.CommandContext(ctx, generator[0], args...) //nolint:gosec // the generator command comes from the Taskfile
	cmd.Dir = filepath.Join(root, filepath.FromSlash(workdir))
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("run generator: %w", err)
	}

	var stale []string
	for _, p := range paths {
		diffs, err := compareTrees(filepath.Join(root, filepath.FromSlash(p)), filepath.Join(scratch, filepath.FromSlash(p)))
		if err != nil {
			return nil, err
		}
		for _, d := range diffs {
			stale = append(stale, filepath.ToSlash(filepath.Join(p, d)))
		}
	}
	return stale, nil
}

// compareTrees returns the slash-separated relative paths that differ between
// the two directories, annotated with how they differ.
func compareTrees(have, want string) ([]string, error) {
	haveFiles, err := listFiles(have)
	if err != nil {
		return nil, err
	}
	wantFiles, err := listFiles(want)
	if err != nil {
		return nil, err
	}

	var diffs []string
	for rel := range wantFiles {
		if _, ok := haveFiles[rel]; !ok {
			diffs = append(diffs, rel+" (missing)")
			continue
		}
		same, err := sameContent(filepath.Join(have, rel), filepath.Join(want, rel))
		if err != nil {
			return nil, err
		}
		if !same {
			diffs = append(diffs, rel+" (changed)")
		}
	}
	for rel := range haveFiles {
		if _, ok := wantFiles[rel]; !ok {
			diffs = append(diffs, rel+" (no longer generated)")
		}
	}
	slices.Sort(diffs)
	return diffs, nil
}

func listFiles(dir string) (map[string]struct{}, error) {
	files := map[string]struct{}{}
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) && path == dir {
				return fs.SkipAll
			}
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		files[filepath.ToSlash(rel)] = struct{}{}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("list %s: %w", dir, err)
	}
	return files, nil
}

func sameContent(a, b string) (bool, error) {
	ab, err := os.ReadFile(a)
	if err != nil {
		return false, fmt.Errorf("read %s: %w", a, err)
	}
	bb, err := os.ReadFile(b)
	if err != nil {
		return false, fmt.Errorf("read %s: %w", b, err)
	}
	return bytes.Equal(ab, bb), nil
}
