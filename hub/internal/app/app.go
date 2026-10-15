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
	"path/filepath"
	"time"

	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/adapter/clock"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/adapter/mail"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/adapter/password"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/adapter/ratelimit"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/adapter/secrets"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/adapter/snowflake"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/adapter/sqlite"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/adapter/token"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/buildinfo"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/config"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/service/account"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/service/auth"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/service/health"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/service/instance"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/service/invite"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/service/notify"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/service/settings"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/service/setup"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/transport/clientip"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/transport/rpc"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/transport/web"
	"github.com/tornellihenrique/dislackso-rebirth/hub/webdist"
)

const (
	readHeaderTimeout = 10 * time.Second
	idleTimeout       = 2 * time.Minute
	shutdownTimeout   = 10 * time.Second
	// Each concurrent password hash holds 64 MiB.
	maxConcurrentHashes = 4
)

type App struct {
	cfg      config.Config
	logger   *slog.Logger
	db       *sqlite.DB
	settings *settings.Service
	setup    *setup.Service
	server   *http.Server
	listener net.Listener
	devMail  bool
}

func New(ctx context.Context, cfg config.Config, logger *slog.Logger) (*App, error) {
	files, err := webFiles(cfg.Web)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(cfg.DataDir, 0o700); err != nil {
		return nil, fmt.Errorf("create data dir: %w", err)
	}
	db, err := sqlite.Open(ctx, filepath.Join(cfg.DataDir, "data.db"))
	if err != nil {
		return nil, err
	}
	a, err := wire(cfg, logger, db, files)
	if err != nil {
		_ = db.Close()
		return nil, err
	}
	return a, nil
}

func wire(cfg config.Config, logger *slog.Logger, db *sqlite.DB, files fs.FS) (*App, error) {
	secretStore, err := secrets.OpenFile(filepath.Join(cfg.DataDir, "secrets.yaml"))
	if err != nil {
		return nil, err
	}
	clk := clock.System{}
	ids, err := snowflake.New(clk, 0)
	if err != nil {
		return nil, err
	}
	tokens := token.Random{}
	limiter := ratelimit.NewMemory(clk)
	hasher := password.NewArgon2id(password.DefaultParams, maxConcurrentHashes)

	settingsSvc := settings.New(db, secretStore, clk)
	mailer := &mail.Router{Config: settingsSvc, SMTP: mail.SMTP{}}
	if cfg.Mail.DevLog {
		mailer.DevLog = &mail.Log{Logger: logger}
	}
	notifier := &notify.Notifier{Mailer: mailer, Settings: settingsSvc}

	invites := invite.New(invite.Deps{
		Tokens: db, Users: db, TokenGen: tokens, IDs: ids, Clock: clk, Settings: settingsSvc,
	})
	authSvc := auth.New(auth.Deps{
		Users: db, Sessions: db, Tokens: db, Tx: db,
		Hasher: hasher, TokenGen: tokens, IDs: ids, Clock: clk, Limiter: limiter,
		Mail: mailer, Settings: settingsSvc, Emails: notifier, Invites: invites, Logger: logger,
	})
	accountSvc := account.New(account.Deps{
		Users: db, Sessions: db, Tx: db, Hasher: hasher, Clock: clk, Limiter: limiter, Verification: authSvc,
	})
	instanceSvc := instance.New(instance.Deps{
		Users: db, Sessions: db, Tx: db, Clock: clk,
		Settings: settingsSvc, ResetLinks: authSvc, Emails: notifier,
	})
	setupToken := cfg.Setup.Token
	if setupToken == "" {
		if setupToken, err = tokens.NewToken(); err != nil {
			return nil, err
		}
	}
	setupSvc := setup.New(setup.Deps{
		Users: db, Tx: db, Hasher: hasher, IDs: ids, Clock: clk, Limiter: limiter,
		Settings: settingsSvc, Sessions: authSvc, Mail: mail.SMTP{}, Token: setupToken,
	})
	healthSvc := health.NewService(clk, buildinfo.Version(), db)

	mux := http.NewServeMux()
	rpc.Mount(mux, rpc.Deps{
		Health:        healthSvc,
		Auth:          authSvc,
		Authenticator: authSvc,
		Account:       accountSvc,
		Instance:      instanceSvc,
		Invites:       invites,
		Setup:         setupSvc,
		ClientIP:      clientip.RemoteAddr,
		Logger:        logger,
	})
	web.Register(mux, web.Options{Files: files, Health: healthSvc, Setup: setupSvc, ClientIP: clientip.RemoteAddr})

	server := &http.Server{
		Handler:           web.AccessLog(logger)(web.SecurityHeaders(mux)),
		ReadHeaderTimeout: readHeaderTimeout,
		IdleTimeout:       idleTimeout,
		ErrorLog:          slog.NewLogLogger(logger.Handler(), slog.LevelWarn),
	}
	return &App{
		cfg: cfg, logger: logger, db: db, settings: settingsSvc, setup: setupSvc,
		server: server, devMail: cfg.Mail.DevLog,
	}, nil
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
	if err := a.announce(ctx, addr); err != nil {
		return err
	}
	return a.Serve(ctx)
}

// announce tells the host what to do next: open the setup wizard, or
// configure email.
func (a *App) announce(ctx context.Context, addr net.Addr) error {
	pending, err := a.setup.Pending(ctx)
	if err != nil {
		return err
	}
	if pending {
		port := 0
		if tcp, ok := addr.(*net.TCPAddr); ok {
			port = tcp.Port
		}
		a.logger.WarnContext(ctx, "setup required: open the setup wizard on this machine",
			slog.String("url", fmt.Sprintf("http://localhost:%d/setup?token=%s", port, a.setup.Token())))
		return nil
	}
	st, err := a.settings.Get(ctx)
	if err != nil {
		return err
	}
	if !st.SMTP.Enabled && !a.devMail {
		a.logger.WarnContext(ctx, "email is not configured: verification is off and password resets need an admin link")
	}
	return nil
}

func (a *App) Close() error { return a.db.Close() }
