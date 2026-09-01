package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"

	charmlog "github.com/charmbracelet/log"
	"github.com/eulerbutcooler/surang/internal/client"
)

var version = "dev"

func main() {
	if len(os.Args) > 1 && os.Args[1] == "login" {
		if err := runLogin(); err != nil {
			fmt.Fprintln(os.Stderr, "surang-client:", err)
			os.Exit(1)
		}
		return
	}

	showVersion := flag.Bool("version", false, "print version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Println(version)
		return
	}

	cfg, err := client.LoadConfig()
	if err != nil {
		fmt.Fprintln(os.Stderr, "surang-client: no token found - run `surang-client login` first")
		os.Exit(1)
	}
	server := flag.String("server", "", "control address override (plaintext, local dev only)")
	var tunnels []string
	flag.Func("tunnel", "label=host:port to expose (repeatable)", func(s string) error {
		tunnels = append(tunnels, s)
		return nil
	})

	if len(tunnels) == 0 {
		fmt.Fprintln(os.Stderr, "surang-client: at least one -tunnel required: -tunnel web=localhost:3000")
		os.Exit(2)
	}

	logger := slog.New(charmlog.NewWithOptions(os.Stderr, charmlog.Options{
		ReportTimestamp: true,
	}))

	parsed, err := client.ParseTunnels(tunnels)
	if err != nil {
		logger.Error("invalid -tunnel flag", "err", err)
		os.Exit(1)
	}

	controlAddr, useTLS, err := client.ControlFromAPI(cfg.API)
	if err != nil {
		logger.Error("control address", "err", err)
		os.Exit(1)
	}
	if *server != "" {
		controlAddr, useTLS = *server, false
	}
	cl := client.NewClient(controlAddr, useTLS, cfg.Token, parsed, logger)
	if err := cl.Connect(context.Background()); err != nil {
		logger.Error("connect failed", "err", err)
		os.Exit(1)
	}
	if err := cl.Serve(); err != nil {
		logger.Error("serve failed", "err", err)
		os.Exit(1)
	}
}
