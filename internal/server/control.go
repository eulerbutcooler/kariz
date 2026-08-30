package server

import (
	"bufio"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"log/slog"
	"net"

	"github.com/eulerbutcooler/surang/internal/auth"
	"github.com/eulerbutcooler/surang/internal/certs"
	"github.com/eulerbutcooler/surang/internal/tunnel"
	"github.com/hashicorp/yamux"
)

type Control struct {
	addr    string
	domain  string
	auth    auth.Authenticator
	reg     *Registry
	certMgr *certs.Manager
	log     *slog.Logger
}

func NewControl(addr, domain string, auth auth.Authenticator, reg *Registry, certMgr *certs.Manager, log *slog.Logger) *Control {
	return &Control{
		addr:    addr,
		domain:  domain,
		auth:    auth,
		reg:     reg,
		certMgr: certMgr,
		log:     log,
	}
}

func (c *Control) Run(ctx context.Context) error {
	ln, err := net.Listen("tcp", c.addr)
	if err != nil {
		return fmt.Errorf("control: listen %w", err)
	}
	// TLS wraps every accepted conn when a cert manager is wired in;
	// yamux runs on top of the decrypted stream, unchanged.
	if c.certMgr != nil {
		ln = tls.NewListener(ln, &tls.Config{
			GetCertificate: c.certMgr.GetCertificate,
			MinVersion:     tls.VersionTLS12,
		})
	}
	go func() {
		<-ctx.Done()
		ln.Close()
	}()
	for {
		conn, err := ln.Accept()
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return fmt.Errorf("control: accept: %w", err)
		}
		go c.handleConn(ctx, conn)
	}
}

func (c *Control) handleConn(ctx context.Context, conn net.Conn) {
	sess, err := yamux.Server(conn, nil)
	if err != nil {
		c.log.Error("yamux handshake failed", "err", err)
		conn.Close()
		return
	}
	defer sess.Close()
	defer func() {
		removed := c.reg.Unregister(sess)
		if len(removed) > 0 {
			c.log.Info("tunnels released", "subdomains", removed)
		}
	}()
	ctrl, err := sess.AcceptStream()
	if err != nil {
		c.log.Error("accept control stream failed", "err", err)
		return
	}
	r := bufio.NewReader(ctrl)

	var reg tunnel.Register
	if err := tunnel.ReadFrame(r, &reg); err != nil {
		c.log.Error("bad register frame", "err", err)
		return
	}
	_, err = c.auth.Authenticate(reg.Secret)
	if err != nil {
		msg := "authentication failed"
		if errors.Is(err, auth.ErrTokenExpired) {
			msg = "token expired"
		}
		tunnel.WriteFrame(ctrl, tunnel.Ack{Error: msg})
		c.log.Error("auth failed", "err", err)
		return
	}
	results := make([]tunnel.TunnelResult, 0, len(reg.Tunnels))
	for _, t := range reg.Tunnels {
		result := c.bindTunnel(sess, t.ID)
		results = append(results, result)
	}
	tunnel.WriteFrame(ctrl, tunnel.Ack{Results: results})
	c.log.Info("client registered", "tunnels", len(reg.Tunnels))
	<-sess.CloseChan()
}

func (c *Control) bindTunnel(sess *yamux.Session, label string) tunnel.TunnelResult {
	for range 10 {
		sub := GenerateID()
		if c.reg.Register(sess, sub, label) {
			return tunnel.TunnelResult{
				ID:   label,
				Host: sub + "." + c.domain,
			}
		}
	}
	return tunnel.TunnelResult{ID: label, Error: "could not allocate subdomain"}
}
