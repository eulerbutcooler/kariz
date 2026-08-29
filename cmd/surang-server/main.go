package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	charmlog "github.com/charmbracelet/log"
	"github.com/eulerbutcooler/surang/internal/api"
	"github.com/eulerbutcooler/surang/internal/auth/token"
	"github.com/eulerbutcooler/surang/internal/server"
	"github.com/eulerbutcooler/surang/internal/store/sqlite"
)

var version = "dev"

func main() {
	controlAddr := flag.String("control", ":5555", "address for client control connections")
	httpAddr := flag.String("http", ":8080", "address for public HTTP traffic")
	domain := flag.String("domain", "surang.online", "wildcard domain suffix for tunnels")
	db := flag.String("db", "chooha.db", "database for surang")
	apiAddr := flag.String("api", ":9000", "address for the account API")
	showVersion := flag.Bool("version", false, "print version and exit")
	flag.Parse()
	if *showVersion {
		fmt.Println(version)
		return
	}

	cfg := server.Config{
		ControlAddr: *controlAddr,
		HTTPAddr:    *httpAddr,
		Domain:      *domain,
	}
	logger := slog.New(charmlog.NewWithOptions(os.Stderr, charmlog.Options{
		ReportTimestamp: true,
	}))

	st, err := sqlite.NewStore(*db)
	if err != nil {
		logger.Error("open store", "err", err)
		os.Exit(1)
	}
	acc := api.NewAPI(*apiAddr, st, logger)
	auth := token.New(st)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	srv := server.New(cfg, auth, acc, logger)
	err = srv.Run(ctx)
	if err != nil {
		logger.Error("server exited with error", "err", err)
		os.Exit(1)
	}
	logger.Info("server stopped")
}
