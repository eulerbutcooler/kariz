package client

import (
	"fmt"
	"net"
	"strings"
)

/*
 * NOTE: kariz-client -server 1.2.3.4:5555 -tunnel web=localhost:3000 -tunnel api=localhost:8000
 */

type Tunnel struct {
	Label string // "web" - becomes Register.Tunnels[].ID
	Local string // "localhost:3000"
}

func ParseTunnels(raw []string) ([]Tunnel, error) {
	tunnels := make([]Tunnel, 0, len(raw))
	for _, s := range raw {
		t, err := parseTunnel(s)
		if err != nil {
			return nil, err
		}
		tunnels = append(tunnels, t)
	}
	return tunnels, nil
}

func parseTunnel(s string) (Tunnel, error) {
	parts := strings.Split(s, "=")
	if len(parts) != 2 {
		return Tunnel{}, fmt.Errorf("client: bad tunnel %q: want label=host:port", s)
	}
	label, local := parts[0], parts[1]
	if label == "" {
		return Tunnel{}, fmt.Errorf("client: bad tunnel %q: empty label", s)
	}
	if local == "" {
		return Tunnel{}, fmt.Errorf("client: bad tunnel %q: empty local target", s)
	}
	if _, _, err := net.SplitHostPort(local); err != nil {
		return Tunnel{}, fmt.Errorf("client: bad tunnel %q: %w", s, err)
	}
	return Tunnel{Label: label, Local: local}, nil
}
