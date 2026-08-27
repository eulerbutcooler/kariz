package server

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/eulerbutcooler/kariz/internal/auth"
)

type Config struct {
	ControlAddr string
	HTTPAddr    string
	Domain      string
}

type Server struct {
	control *Control
	public  *Public
}

func New(cfg Config, auth auth.Authenticator, log *slog.Logger) *Server {
	reg := NewRegistry()
	control := NewControl(cfg.ControlAddr, cfg.Domain, auth, reg, log)
	public := NewPublic(cfg.HTTPAddr, cfg.Domain, reg, log)
	return &Server{
		control: control,
		public:  public,
	}
}

func (s *Server) Run(ctx context.Context) error {
	loopCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	/* NOTE: Buffer of 2 because we can then take in errros of the losing go routine
	* after we take in the error of the go routine that returns first. so the losing
	* one doesnt send without a receiver and hang forever.
	 */
	errCh := make(chan error, 2)
	go func() {
		errCh <- s.public.Run(loopCtx)
	}()
	go func() {
		errCh <- s.control.Run(loopCtx)
	}()
	err := <-errCh
	cancel()
	return fmt.Errorf("server: %w", err)
}
