package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/eulerbutcooler/kariz/internal/auth/allowall"
	"github.com/eulerbutcooler/kariz/internal/server"
)

func main() {
	controlAddr := flag.String("control", ":8000", "address for client control connections")
	httpAddr := flag.String("http", ":8080", "address for public HTTP traffic")
	domain := flag.String("domain", "kariz.xyz", "wildcard domain suffix for tunnels")
	flag.Parse()

	cfg := server.Config{
		ControlAddr: *controlAddr,
		HTTPAddr:    *httpAddr,
		Domain:      *domain,
	}
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	auth := allowall.NewAllowAll()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	srv := server.New(cfg, auth, logger)
	err := srv.Run(ctx)
	if err != nil {
		logger.Error("server exited with error", "err", err)
		os.Exit(1)
	}
	logger.Info("server stopped")
}
