// Package app is the composition root: the only package that knows the
// concrete adapters and wires them into services and transports.
package app

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/adapter/clock"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/buildinfo"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/config"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/service/health"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/transport/rpc"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/transport/web"
	"github.com/tornellihenrique/dislackso-rebirth/hub/webdist"
)

const (
	readHeaderTimeout = 10 * time.Second
	idleTimeout       = 2 * time.Minute
	shutdownTimeout   = 10 * time.Second
)

type App struct {
	cfg      config.Config
	logger   *slog.Logger
	server   *http.Server
	listener net.Listener
}

func New(cfg config.Config, logger *slog.Logger) (*App, error) {
	files, err := webFiles(cfg.Web)
	if err != nil {
		return nil, err
	}

	healthSvc := health.NewService(clock.System{}, buildinfo.Version())

	mux := http.NewServeMux()
	rpc.Mount(mux, rpc.Services{
		Health: rpc.NewHealthHandler(healthSvc),
	})
	web.Register(mux, web.Options{Files: files, Health: healthSvc})

	server := &http.Server{
		Handler:           web.AccessLog(logger)(web.SecurityHeaders(mux)),
		ReadHeaderTimeout: readHeaderTimeout,
		IdleTimeout:       idleTimeout,
		ErrorLog:          slog.NewLogLogger(logger.Handler(), slog.LevelWarn),
	}
	return &App{cfg: cfg, logger: logger, server: server}, nil
}

func webFiles(cfg config.Web) (fs.FS, error) {
	if cfg.Dir == "" {
		return webdist.FS()
	}
	info, err := os.Stat(cfg.Dir)
	if err != nil {
		return nil, fmt.Errorf("web.dir: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("web.dir %s is not a directory", cfg.Dir)
	}
	return os.DirFS(cfg.Dir), nil
}

// Listen binds the HTTP listener, so a port conflict is reported before
// anything else starts.
func (a *App) Listen(ctx context.Context) (net.Addr, error) {
	var lc net.ListenConfig
	ln, err := lc.Listen(ctx, "tcp", a.cfg.HTTP.Addr)
	if err != nil {
		return nil, fmt.Errorf("listen on %s: %w", a.cfg.HTTP.Addr, err)
	}
	a.listener = ln
	return ln.Addr(), nil
}

// Serve blocks until ctx is cancelled, then drains in-flight requests.
func (a *App) Serve(ctx context.Context) error {
	if a.listener == nil {
		return errors.New("serve called before listen")
	}
	serveErr := make(chan error, 1)
	go func() { serveErr <- a.server.Serve(a.listener) }()

	select {
	case err := <-serveErr:
		return fmt.Errorf("serve http: %w", err)
	case <-ctx.Done():
	}

	a.logger.InfoContext(ctx, "shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), shutdownTimeout)
	defer cancel()
	if err := a.server.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shut down http: %w", err)
	}
	if err := <-serveErr; !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("serve http: %w", err)
	}
	return nil
}

// Run listens and serves until ctx is cancelled.
func (a *App) Run(ctx context.Context) error {
	addr, err := a.Listen(ctx)
	if err != nil {
		return err
	}
	a.logger.InfoContext(ctx, "hub listening",
		slog.String("url", "http://"+addr.String()),
		slog.String("version", buildinfo.Version()),
		slog.String("data_dir", a.cfg.DataDir),
	)
	return a.Serve(ctx)
}
