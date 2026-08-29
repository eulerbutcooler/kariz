package client

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"net"

	"github.com/eulerbutcooler/surang/internal/tunnel"
	"github.com/hashicorp/yamux"
)

var ErrNotConnected = errors.New("client: not connected; call Connect first")

func (c *Client) Serve() error {
	if c.sess == nil {
		return ErrNotConnected
	}
	for {
		stream, err := c.sess.AcceptStream()
		if err != nil {
			return fmt.Errorf("cannot accept the stream: %w", err)
		}
		go c.handleStream(stream)
	}
}

func (c *Client) handleStream(stream *yamux.Stream) {
	r := bufio.NewReader(stream)
	var dial tunnel.Dial
	if err := tunnel.ReadFrame(r, &dial); err != nil {
		c.log.Error("unable to read tunnelID", "err", err)
		stream.Close()
		return
	}
	local, ok := c.tunnels[dial.TunnelID]
	if !ok {
		c.log.Error("no tunnel with that id found", "label", dial.TunnelID)
		stream.Close()
		return
	}
	conn, err := net.Dial("tcp", local.Local)
	if err != nil {
		c.log.Error("dial local service", "label", dial.TunnelID, "local", local.Local, "err", err)
		stream.Close()
		return
	}
	go func() {
		io.Copy(conn, r)
		if tcp, ok := conn.(*net.TCPConn); ok {
			tcp.CloseWrite()
		}
	}()
	io.Copy(stream, conn)
	conn.Close()
	stream.Close()
}
