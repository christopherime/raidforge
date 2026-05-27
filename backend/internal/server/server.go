// Package server implements the raidforge HTTP server: routing, middleware, and
// lifecycle. The API surface (docs/SPEC.md §7) is mounted here in later milestones.
package server

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/christopherime/raidforge/internal/config"
)

// Server wires configuration, logging, and HTTP routes.
type Server struct {
	cfg     *config.Config
	log     *slog.Logger
	version string
}

// New creates a Server.
func New(cfg *config.Config, logger *slog.Logger, version string) *Server {
	return &Server{cfg: cfg, log: logger, version: version}
}

// handler builds the routed, middleware-wrapped HTTP handler.
func (s *Server) handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.handleHealthz)
	// TODO(v0.4.0): mount /api/tiers, /api/optimize, ... here.
	return s.recoverer(s.requestLogger(mux))
}

// Run starts the HTTP server and blocks until ctx is cancelled, then drains
// connections within the configured shutdown grace period.
func (s *Server) Run(ctx context.Context) error {
	httpSrv := &http.Server{
		Addr:              s.cfg.Server.Addr,
		Handler:           s.handler(),
		ReadHeaderTimeout: s.cfg.Server.ReadTimeout,
		ReadTimeout:       s.cfg.Server.ReadTimeout,
		WriteTimeout:      s.cfg.Server.WriteTimeout,
		IdleTimeout:       s.cfg.Server.IdleTimeout,
	}

	errc := make(chan error, 1)
	go func() {
		s.log.Info("raidforge starting", "addr", s.cfg.Server.Addr, "version", s.version)
		if err := httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errc <- err
		}
	}()

	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
		s.log.Info("shutdown signal received, draining connections")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), s.cfg.Server.ShutdownGrace)
	defer cancel()
	return httpSrv.Shutdown(shutdownCtx)
}

func (s *Server) handleHealthz(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok\n"))
}
