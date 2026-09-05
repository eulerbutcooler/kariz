package server

import (
	"bufio"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httputil"
	"strings"
	"time"

	"github.com/eulerbutcooler/surang/internal/certs"
	"github.com/eulerbutcooler/surang/internal/tunnel"
	"github.com/hashicorp/yamux"
)

/* NOTE: Public facing half of the server. Listens to curl and browser traffic on
 * the HTTP port and routes each request into the yamux session that owns the subdomain
 * from the 'Host' header.
 * ReverseProxy handles the HTTP mechanics
 * The tunnel doesnt parse the HTTP semantics it just passes the request down a stream
 * and the response back up
 */
type Public struct {
	addr    string
	domain  string
	reg     *Registry
	certMgr *certs.Manager
	api     http.Handler
	log     *slog.Logger
	proxy   *httputil.ReverseProxy
}

type ctxKey int

const bindingKey ctxKey = 0

func NewPublic(addr, domain string, reg *Registry, certMgr *certs.Manager, api http.Handler, log *slog.Logger) *Public {
	p := &Public{
		addr:    addr,
		domain:  domain,
		reg:     reg,
		log:     log,
		certMgr: certMgr,
		api:     api,
	}
	p.proxy = &httputil.ReverseProxy{
		Rewrite: func(pr *httputil.ProxyRequest) {
			u := *pr.In.URL
			u.Scheme = "http"
			u.Host = "tunnel.internal"
			pr.Out.URL = &u
			pr.Out.Host = pr.In.Host
			pr.SetXForwarded()
		},
		Transport: &tunnelTransport{},
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			log.Error("proxy error", "err", err, "path", r.URL.Path)
			http.Error(w, "bad gateway", http.StatusBadGateway)
		},
		FlushInterval: -1,
	}
	return p
}

func (p *Public) runTLS(ctx context.Context) error {
	srv := &http.Server{
		Addr:    ":443",
		Handler: p,
		TLSConfig: &tls.Config{
			GetCertificate: p.certMgr.GetCertificate,
			MinVersion:     tls.VersionTLS12,
		},
	}
	redir := &http.Server{
		Addr: ":80",
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, "https://"+r.Host+r.RequestURI, http.StatusMovedPermanently)
		}),
	}
	go func() {
		err := redir.ListenAndServe()
		if err != nil && err != http.ErrServerClosed {
			p.log.Error("redirect listener failed", "err", err)
		}
	}()

	errCh := make(chan error, 1)
	go func() { errCh <- srv.ListenAndServeTLS("", "") }()
	select {
	case err := <-errCh:
		return fmt.Errorf("public: %w", err)
	case <-ctx.Done():
		ctx2, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return errors.Join(srv.Shutdown(ctx2), redir.Shutdown(ctx2))
	}
}

func (p *Public) Run(ctx context.Context) error {
	if p.certMgr == nil {
		return p.runPlaintext(ctx)
	}
	return p.runTLS(ctx)
}

func (p *Public) runPlaintext(ctx context.Context) error {
	srv := &http.Server{
		Addr:    p.addr,
		Handler: p,
	}
	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.ListenAndServe()
	}()
	select {
	case err := <-errCh:
		return fmt.Errorf("public: %w", err)
	case <-ctx.Done():
		ctx2, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return srv.Shutdown(ctx2)
	}
}

func (p *Public) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	host := r.Host
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	sub := strings.TrimSuffix(host, "."+p.domain)
	if sub == host || sub == "" {
		http.Error(w, "no such tunnel", http.StatusNotFound)
		return
	}
	// api.<domain> serves the account API on the same listener as the
	// tunnels; "api" can never collide with a generated 3-word ID.
	if sub == "api" {
		p.api.ServeHTTP(w, r)
		return
	}
	binding, ok := p.reg.Lookup(sub)
	if !ok {
		http.Error(w, "no such tunnel", http.StatusNotFound)
		return
	}
	ctx := context.WithValue(r.Context(), bindingKey, binding)
	p.proxy.ServeHTTP(w, r.WithContext(ctx))
	p.log.Info("proxied", "sub", sub, "label", binding.Label,
		"method", r.Method, "path", r.URL.Path)
}

type tunnelTransport struct{}

var _ http.RoundTripper = (*tunnelTransport)(nil)

func (t *tunnelTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	binding, ok := req.Context().Value(bindingKey).(Binding)
	if !ok {
		return nil, errors.New("no tunnel binding in request context")
	}
	stream, err := binding.Session.OpenStream()
	if err != nil {
		return nil, fmt.Errorf("open stream: %w", err)
	}
	idle := &idleConn{Conn: stream, timeout: 30 * time.Second}
	idle.bump()

	if err := tunnel.WriteFrame(idle, tunnel.Dial{TunnelID: binding.Label}); err != nil {
		stream.Close()
		return nil, fmt.Errorf("write dial: %w", err)
	}
	go func() {
		if err := req.Write(idle); err != nil {
			stream.Close()
		}
	}()
	br := bufio.NewReader(idle)
	resp, err := http.ReadResponse(br, req)
	for err == nil && resp.StatusCode == http.StatusContinue {
		resp.Body.Close()
		resp, err = http.ReadResponse(br, req)
	}
	if err != nil {
		stream.Close()
		return nil, fmt.Errorf("read response: %w", err)
	}
	resp.Body = &streamBody{ReadCloser: resp.Body, stream: stream}
	return resp, nil
}

type streamBody struct {
	io.ReadCloser
	stream *yamux.Stream
}

func (b *streamBody) Close() error {
	bodyErr := b.ReadCloser.Close()
	streamErr := b.stream.Close()
	if bodyErr != nil {
		return bodyErr
	}
	return streamErr
}

type idleConn struct {
	net.Conn
	timeout time.Duration
}

func (c *idleConn) bump() {
	d := time.Now().Add(c.timeout)
	c.Conn.SetReadDeadline(d)
	c.Conn.SetWriteDeadline(d)
}

func (c *idleConn) Read(p []byte) (int, error) {
	n, err := c.Conn.Read(p)
	if n > 0 {
		c.bump()
	}
	return n, err
}

func (c *idleConn) Write(p []byte) (int, error) {
	n, err := c.Conn.Write(p)
	if n > 0 {
		c.bump()
	}
	return n, err
}
