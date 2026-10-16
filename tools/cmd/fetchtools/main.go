// Command fetchtools installs the pinned developer tools listed in tools/manifest.json.
//
// Usage: fetchtools [-manifest path] [-bin dir] [-force] [tool...]
//
// With no tool names every tool in the manifest is installed.
package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"time"
)

func main() {
	manifestPath := flag.String("manifest", "tools/manifest.json", "path to the tool manifest")
	binDir := flag.String("bin", ".tools/bin", "directory the tools are installed into")
	force := flag.Bool("force", false, "reinstall even when the pinned version is already present")
	platform := flag.String("platform", runtime.GOOS+"/"+runtime.GOARCH, "goos/goarch to fetch for (release builds)")
	flag.Parse()

	if err := runWithSignals(*manifestPath, *binDir, *platform, *force, flag.Args()); err != nil {
		fmt.Fprintln(os.Stderr, "fetchtools:", err)
		os.Exit(1)
	}
}

func runWithSignals(manifestPath, binDir, platform string, force bool, names []string) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	return run(ctx, manifestPath, binDir, platform, force, names)
}

func run(ctx context.Context, manifestPath, binDir, platform string, force bool, names []string) error {
	m, err := loadManifest(manifestPath)
	if err != nil {
		return err
	}
	tools, err := m.selectTools(names)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(binDir, 0o750); err != nil {
		return fmt.Errorf("create %s: %w", binDir, err)
	}

	in := installer{
		client:   &http.Client{Timeout: 10 * time.Minute},
		binDir:   binDir,
		platform: platform,
		exeExt:   exeExt(strings.SplitN(platform, "/", 2)[0]),
		force:    force,
		log:      os.Stdout,
	}
	for _, t := range tools {
		if err := in.install(ctx, t); err != nil {
			return fmt.Errorf("install %s %s: %w", t.Name, t.Version, err)
		}
	}
	return nil
}

func exeExt(goos string) string {
	if goos == "windows" {
		return ".exe"
	}
	return ""
}
