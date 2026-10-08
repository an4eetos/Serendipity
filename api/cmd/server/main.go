// Command server runs the Serendipity API.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/an4eetos/serendipity/api/internal/auth"
	"github.com/an4eetos/serendipity/api/internal/config"
	"github.com/an4eetos/serendipity/api/internal/llm"
	"github.com/an4eetos/serendipity/api/internal/openlibrary"
	"github.com/an4eetos/serendipity/api/internal/server"
	"github.com/an4eetos/serendipity/api/internal/storage"
	"github.com/an4eetos/serendipity/api/internal/store"
)

func main() {
	if err := run(); err != nil {
		slog.Error("server stopped", "err", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	st, err := store.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer st.Close()

	srv := server.New(
		st,
		auth.NewVerifier(ctx, cfg.SupabaseURL, cfg.SupabaseJWTSecret),
		llm.New(cfg.AnthropicModel),
		openlibrary.New(),
		storage.New(cfg.SupabaseURL, cfg.SupabaseServiceRoleKey),
	)
	httpServer := &http.Server{
		Addr:              cfg.Addr,
		Handler:           srv.Routes(cfg.AllowedOrigins),
		ReadHeaderTimeout: 10 * time.Second,
	}

	errc := make(chan error, 1)
	go func() {
		slog.Info("listening", "addr", cfg.Addr, "model", cfg.AnthropicModel)
		errc <- httpServer.ListenAndServe()
	}()
	select {
	case err := <-errc:
		if !errors.Is(err, http.ErrServerClosed) {
			return err
		}
	case <-ctx.Done():
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return httpServer.Shutdown(shutdownCtx)
}
