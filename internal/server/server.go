package server

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/eulerbutcooler/kariz/internal/api"
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
	api     *api.API
}

func New(cfg Config, auth auth.Authenticator, acc *api.API, log *slog.Logger) *Server {
	reg := NewRegistry()
	control := NewControl(cfg.ControlAddr, cfg.Domain, auth, reg, log)
	public := NewPublic(cfg.HTTPAddr, cfg.Domain, reg, log)
	return &Server{
		control: control,
		public:  public,
		api:     acc,
	}
}

func (s *Server) Run(ctx context.Context) error {
	loopCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	/* NOTE: Buffer of 3 because we can then take in errros of the losing go routine
	* after we take in the error of the go routine that returns first. so the losing
	* one doesnt send without a receiver and hang forever.
	 */
	errCh := make(chan error, 3)
	go func() {
		errCh <- s.public.Run(loopCtx)
	}()
	go func() {
		errCh <- s.control.Run(loopCtx)
	}()
	go func() {
		errCh <- s.api.Run(loopCtx)
	}()
	err := <-errCh
	cancel()
	if err != nil {
		return fmt.Errorf("server: %w", err)
	}
	return nil
}
