package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"

	"github.com/eulerbutcooler/kariz/internal/client"
)

func main() {
	server := flag.String("server", "localhost:5555", "kariz server control address")
	secret := flag.String("secret", "", "tunnel token (server must allow it)")
	var tunnels []string
	flag.Func("tunnel", "label=host:port to expose (repeatable)", func(s string) error {
		tunnels = append(tunnels, s)
		return nil
	})
	flag.Parse()

	if len(tunnels) == 0 {
		fmt.Fprintln(os.Stderr, "kariz-client: at least one -tunnel required: -tunnel web=localhost:3000")
		os.Exit(2)
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	parsed, err := client.ParseTunnels(tunnels)
	if err != nil {
		logger.Error("invalid -tunnel flag", "err", err)
		os.Exit(1)
	}

	cl := client.NewClient(*server, *secret, parsed, logger)
	if err := cl.Connect(context.Background()); err != nil {
		logger.Error("connect failed", "err", err)
		os.Exit(1)
	}
	if err := cl.Serve(); err != nil {
		logger.Error("serve failed", "err", err)
		os.Exit(1)
	}
}
