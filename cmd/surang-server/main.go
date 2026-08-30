package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	charmlog "github.com/charmbracelet/log"
	"github.com/eulerbutcooler/surang/internal/api"
	"github.com/eulerbutcooler/surang/internal/auth/token"
	"github.com/eulerbutcooler/surang/internal/certs"
	"github.com/eulerbutcooler/surang/internal/server"
	"github.com/eulerbutcooler/surang/internal/store/sqlite"
)

var version = "dev"

func main() {
	tlsDomain := flag.String("tls-domain", "", "wildcard TLS domain (e.g. surang.eulerbutcooler.xyz); empty = plaintext for local dev")
	tlsEmail := flag.String("tls-email", "", "ACME account email for LetsEncrypt")
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

	logger := slog.New(charmlog.NewWithOptions(os.Stderr, charmlog.Options{
		ReportTimestamp: true,
	}))

	// cert cache lives beside the database so one volume/state dir holds all state
	dbDir := filepath.Dir(*db)

	st, err := sqlite.NewStore(*db)
	if err != nil {
		logger.Error("open store", "err", err)
		os.Exit(1)
	}

	var certManager *certs.Manager
	if *tlsDomain != "" {
		cfToken := os.Getenv("CF_DNS_API_TOKEN")
		if cfToken == "" {
			logger.Error("tls requested but CF_DNS_API_TOKEN is not set")
			os.Exit(1)
		}
		cm, err := certs.New(*tlsDomain, *tlsEmail, cfToken, filepath.Join(dbDir, "certs"), logger)
		if err != nil {
			logger.Error("certs", "err", err)
			os.Exit(1)
		}
		certManager = cm
	}

	acc := api.NewAPI(*apiAddr, st, certManager, logger)
	auth := token.New(st)

	cfg := server.Config{
		ControlAddr: *controlAddr,
		HTTPAddr:    *httpAddr,
		Domain:      *domain,
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	srv := server.New(cfg, auth, certManager, acc, logger)
	err = srv.Run(ctx)
	if err != nil {
		logger.Error("server exited with error", "err", err)
		os.Exit(1)
	}
	logger.Info("server stopped")
}
