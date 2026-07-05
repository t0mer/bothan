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
	"github.com/t0mer/bothan/internal/server"
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

	cfg, err := config.Load(fs)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	logger := newLogger(cfg.Log)
	slog.SetDefault(logger)

	st, err := store.Open(cfg.Database.Path)
	if err != nil {
		return fmt.Errorf("opening store: %w", err)
	}
	defer st.Close()

	m := metrics.New()

	handler, err := server.New(server.Deps{
		Config:  cfg,
		Store:   st,
		Metrics: m,
		Logger:  logger,
	})
	if err != nil {
		return fmt.Errorf("building server: %w", err)
	}

	addr := server.Addr(cfg.Server.Host, cfg.Server.Port)
	httpSrv := &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 1)
	go func() {
		logger.Info("bothan starting",
			slog.String("version", version.Version),
			slog.String("addr", addr),
			slog.String("api_version", cfg.SSLLabs.APIVersion),
		)
		if err := httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		return fmt.Errorf("http server: %w", err)
	case <-ctx.Done():
		logger.Info("shutdown signal received")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := httpSrv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("graceful shutdown: %w", err)
	}
	logger.Info("bothan stopped")
	return nil
}

// newLogger builds a slog logger honoring the configured level and format.
func newLogger(c config.Log) *slog.Logger {
	var level slog.Level
	switch c.Level {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{Level: level}
	var handler slog.Handler
	if c.Format == "text" {
		handler = slog.NewTextHandler(os.Stdout, opts)
	} else {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	}
	return slog.New(handler)
}
