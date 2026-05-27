// Command raidforge runs the raidforge API server: it computes the optimal
// per-boss Mythic raid composition for a World of Warcraft guild's roster.
//
// At this stage (v0.0.x) it serves only a health probe; the domain data,
// optimizer, connectors and API land in later milestones — see docs/SPEC.md
// and TODO.md.
package main

import (
	"context"
	"errors"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

// version is stamped at build time via -ldflags "-X main.version=...".
var version = "dev"

func main() {
	addr := flag.String("addr", ":8080", "HTTP listen address")
	flag.Parse()

	logger := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	if err := run(*addr, logger); err != nil {
		logger.Error("server exited with error", "err", err)
		os.Exit(1)
	}
}

// run starts the HTTP server and blocks until a shutdown signal, then drains
// connections gracefully.
func run(addr string, logger *slog.Logger) error {
	srv := &http.Server{
		Addr:              addr,
		Handler:           newMux(logger),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	errc := make(chan error, 1)
	go func() {
		logger.Info("raidforge starting", "addr", addr, "version", version)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errc <- err
		}
	}()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
		logger.Info("shutdown signal received, draining connections")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return srv.Shutdown(shutdownCtx)
}

// newMux wires the HTTP routes. For now only the Kubernetes health probe;
// the API surface (docs/SPEC.md §7) is mounted here in later milestones.
func newMux(_ *slog.Logger) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok\n"))
	})
	return mux
}
