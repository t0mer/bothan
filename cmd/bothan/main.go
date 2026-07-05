// Command bothan is the single-binary SSL/TLS posture monitor.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/pflag"

	"github.com/t0mer/bothan/internal/config"
	"github.com/t0mer/bothan/internal/metrics"
	"github.com/t0mer/bothan/internal/scanner"
	"github.com/t0mer/bothan/internal/server"
	"github.com/t0mer/bothan/internal/settings"
	"github.com/t0mer/bothan/internal/store"
	"github.com/t0mer/bothan/internal/version"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	fs := pflag.NewFlagSet("bothan", pflag.ContinueOnError)
	config.RegisterFlags(fs)
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, pflag.ErrHelp) {
			return nil
		}
		return err
	}

	if showVersion, _ := fs.GetBool("version"); showVersion {
		fmt.Println(version.Version)
		return nil
	}

	bootstrap, err := config.Load(fs)
	if err != nil {
		return fmt.Errorf("loading bootstrap config: %w", err)
	}

	// A live log level lets the Settings page change verbosity without a restart.
	levelVar := new(slog.LevelVar)
	levelVar.Set(slog.LevelInfo)

	ctx := context.Background()

	st, err := store.Open(bootstrap.DatabasePath)
	if err != nil {
		return fmt.Errorf("opening store: %w", err)
	}
	defer st.Close()

	settingsSvc, err := settings.New(ctx, st.Settings(), *bootstrap)
	if err != nil {
		return fmt.Errorf("loading settings: %w", err)
	}

	logger := newLogger(settingsSvc.Current().Log, levelVar)
	slog.SetDefault(logger)
	settingsSvc.OnChange(func(s *settings.Settings) {
		levelVar.Set(parseLevel(s.Log.Level))
	})

	m := metrics.New()

	scanSvc := scanner.New(scanner.Options{
		Store:    st,
		Settings: settingsSvc,
		Factory:  scanner.DefaultFactory(bootstrap.SSLLabsBaseURL, nil),
		Logger:   logger,
	})
	recoverPendingScans(ctx, st, scanSvc, logger)

	handler, err := server.New(server.Deps{
		Settings: settingsSvc,
		Store:    st,
		Metrics:  m,
		Scanner:  scanSvc,
		Logger:   logger,
	})
	if err != nil {
		return fmt.Errorf("building server: %w", err)
	}

	host, port := settingsSvc.EffectiveBind()
	addr := server.Addr(host, port)
	httpSrv := &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
	}

	sigCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 1)
	go func() {
		logger.Info("bothan starting",
			slog.String("version", version.Version),
			slog.String("addr", addr),
			slog.String("api_version", settingsSvc.Current().SSLLabs.APIVersion),
			slog.Bool("encryption_key_set", bootstrap.EncryptionKey != ""),
		)
		if err := httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		return fmt.Errorf("http server: %w", err)
	case <-sigCtx.Done():
		logger.Info("shutdown signal received")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := httpSrv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("graceful shutdown: %w", err)
	}
	logger.Info("waiting for in-flight scans")
	scanSvc.Wait()
	logger.Info("bothan stopped")
	return nil
}

// recoverPendingScans re-dispatches scans left pending/running by a previous
// process (best-effort restart recovery).
func recoverPendingScans(ctx context.Context, st *store.Store, sc *scanner.Service, logger *slog.Logger) {
	pending, err := st.Scans().PendingScans(ctx)
	if err != nil {
		logger.Error("recovering pending scans", slog.String("error", err.Error()))
		return
	}
	for _, scan := range pending {
		host, err := st.Hosts().Get(ctx, scan.HostID)
		if err != nil {
			continue
		}
		logger.Info("resuming scan", slog.Int64("scan", scan.ID), slog.String("host", host.Hostname))
		sc.Resume(*host, scan.ID)
	}
}

// newLogger builds a slog logger honoring the configured format, with the level
// driven by a shared LevelVar so it can change at runtime.
func newLogger(c settings.LogSettings, level *slog.LevelVar) *slog.Logger {
	level.Set(parseLevel(c.Level))
	opts := &slog.HandlerOptions{Level: level}
	var handler slog.Handler
	if c.Format == "text" {
		handler = slog.NewTextHandler(os.Stdout, opts)
	} else {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	}
	return slog.New(handler)
}

func parseLevel(s string) slog.Level {
	switch s {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
