// Command raidforge runs the raidforge API server: it computes the optimal
// per-boss Mythic raid composition for a World of Warcraft guild's roster.
//
// The domain data, optimizer, connectors and full API land in later milestones
// — see docs/SPEC.md and TODO.md.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/christopherime/raidforge/internal/config"
	"github.com/christopherime/raidforge/internal/logging"
	"github.com/christopherime/raidforge/internal/server"
)

// version is stamped at build time via -ldflags "-X main.version=...".
var version = "dev"

func main() {
	configPath := flag.String("config", "", "path to a YAML config file (empty uses built-in defaults)")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "raidforge: %v\n", err)
		os.Exit(1)
	}

	logger := logging.New(cfg.Logging.Level, cfg.Logging.Format)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := server.New(cfg, logger, version).Run(ctx); err != nil {
		logger.Error("server exited with error", "err", err)
		os.Exit(1)
	}
}
