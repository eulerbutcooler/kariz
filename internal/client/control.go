package client

import (
	"bufio"
	"context"
	"crypto/tls"
	"fmt"
	"log/slog"
	"net"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/eulerbutcooler/surang/internal/tunnel"
	"github.com/hashicorp/yamux"
)

var (
	panelStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("62")).
			Padding(1, 2)

	okStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("42")).
		Bold(true)

	errStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("196")).
			Bold(true)
)

type Client struct {
	server  string
	useTLS  bool
	secret  string
	tunnels map[string]Tunnel
	sess    *yamux.Session
	log     *slog.Logger
}

func NewClient(server string, useTLS bool, secret string, tunnels []Tunnel, log *slog.Logger) *Client {
	byLabel := make(map[string]Tunnel, len(tunnels))
	for _, t := range tunnels {
		byLabel[t.Label] = t
	}
	return &Client{
		server:  server,
		useTLS:  useTLS,
		secret:  secret,
		tunnels: byLabel,
		log:     log,
	}
}

func (c *Client) registerTunnels() []tunnel.Tunnel {
	tunnels := make([]tunnel.Tunnel, 0, len(c.tunnels))
	for label := range c.tunnels {
		tunnels = append(tunnels, tunnel.Tunnel{
			ID:     label,
			Scheme: "http",
		})
	}
	return tunnels
}

func (c *Client) Connect(ctx context.Context) error {
	var conn net.Conn
	var err error
	if c.useTLS {
		host, _, splitErr := net.SplitHostPort(c.server)
		if splitErr != nil {
			host = c.server
		}
		conn, err = tls.Dial("tcp", c.server, &tls.Config{
			ServerName: host,
			MinVersion: tls.VersionTLS12,
		})
	} else {
		conn, err = net.Dial("tcp", c.server)
	}
	if err != nil {
		return fmt.Errorf("dial %s: %w", c.server, err)
	}
	sess, err := yamux.Client(conn, nil)
	if err != nil {
		return fmt.Errorf("yamux client error: %w", err)
	}
	ctrl, err := sess.OpenStream()
	if err != nil {
		return fmt.Errorf("sess.OpenStream %w", err)
	}
	r := bufio.NewReader(ctrl)
	tunnels := c.registerTunnels()
	if err := tunnel.WriteFrame(ctrl, tunnel.Register{Secret: c.secret, Tunnels: tunnels}); err != nil {
		return fmt.Errorf("error when registering: %w", err)
	}
	var ack tunnel.Ack
	if err := tunnel.ReadFrame(r, &ack); err != nil {
		return fmt.Errorf("read ack: %w", err)
	}
	if ack.Error != "" {
		return fmt.Errorf("registration rejected: %s", ack.Error)
	}
	scheme := "http"
	if c.useTLS {
		scheme = "https"
	}
	var rows []string
	for _, res := range ack.Results {
		if res.Error != "" {
			rows = append(rows, errStyle.Render(`¯\_(ツ)_/¯`+res.ID+" "+res.Error))
			continue
		}
		rows = append(rows, okStyle.Render(res.ID)+
			"  →  "+lipgloss.NewStyle().Bold(true).Render(scheme+"://"+res.Host))
	}
	fmt.Println(panelStyle.Render(strings.Join(rows, "\n")))
	c.sess = sess
	return nil
}
