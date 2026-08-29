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

func main() {
	controlAddr := flag.String("control", ":8000", "address for client control connections")
	httpAddr := flag.String("http", ":8080", "address for public HTTP traffic")
	domain := flag.String("domain", "surang.online", "wildcard domain suffix for tunnels")
	db := flag.String("db", "chooha.db", "database for surang")
	addtoken := flag.String("addtoken", "", "admin: mint a token with this label, print once, exit")
	apiAddr := flag.String("api", ":9000", "address for the account API")
	flag.Parse()

	mintMode := false
	flag.Visit(func(f *flag.Flag) {
		if f.Name == "addtoken" {
			mintMode = true
		}
	})

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
	if mintMode {
		if *addtoken == "" {
			logger.Error("mint token", "err", "empty label: -addtoken needs a label")
			os.Exit(1)
		}
		plain, err := token.Mint(st, *addtoken, "")
		if err != nil {
			logger.Error("mint token", "err", err)
			os.Exit(1)
		}
		logger.Info("token minted - show once", "label", *addtoken)
		fmt.Println(plain)
		return
	}
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
