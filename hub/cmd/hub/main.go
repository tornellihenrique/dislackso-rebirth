// Command hub runs a self-hosted instance: API, realtime gateway, web client
// and the media sidecars it supervises.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/app"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/buildinfo"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/config"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/logging"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/product"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	cmd, rest := "run", args
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		cmd, rest = args[0], args[1:]
	}
	switch cmd {
	case "run":
		return runHub(rest, stderr)
	case "version":
		_, _ = fmt.Fprintf(stdout, "%s hub %s\n", product.Name, buildinfo.Version())
		return 0
	case "help":
		usage(stdout)
		return 0
	default:
		_, _ = fmt.Fprintf(stderr, "unknown command %q\n\n", cmd)
		usage(stderr)
		return 2
	}
}

func usage(w io.Writer) {
	_, _ = fmt.Fprintf(w, `%s hub %s

Usage:
  hub [run] [flags]   start the hub (default; "hub run -h" lists flags)
  hub version         print the version
  hub help            show this help
`, product.Name, buildinfo.Version())
}

func runHub(args []string, stderr io.Writer) int {
	cfg, err := config.Load(config.Source{
		Args:           args,
		LookupEnv:      os.LookupEnv,
		DefaultDataDir: config.DefaultDataDir,
		Output:         stderr,
	})
	if errors.Is(err, flag.ErrHelp) {
		return 0
	}
	if err != nil {
		_, _ = fmt.Fprintln(stderr, "hub:", err)
		return 2
	}

	logger, err := logging.New(stderr, cfg.Log.Level, cfg.Log.Format)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, "hub:", err)
		return 2
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	a, err := app.New(cfg, logger)
	if err != nil {
		logger.Error("hub failed to start", "error", err)
		return 1
	}
	if err := a.Run(ctx); err != nil {
		logger.Error("hub stopped with an error", "error", err)
		return 1
	}
	return 0
}
