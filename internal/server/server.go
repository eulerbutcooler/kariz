package server

import (
	"context"
	"log/slog"

	"github.com/eulerbutcooler/surang/internal/api"
	"github.com/eulerbutcooler/surang/internal/auth"
	"github.com/eulerbutcooler/surang/internal/certs"
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

func New(cfg Config, auth auth.Authenticator, certMgr *certs.Manager, acc *api.API, log *slog.Logger) *Server {
	reg := NewRegistry()
	control := NewControl(cfg.ControlAddr, cfg.Domain, auth, reg, certMgr, log)
	public := NewPublic(cfg.HTTPAddr, cfg.Domain, reg, certMgr, log)
	return &Server{
		control: control,
		public:  public,
		api:     acc,
	}
}

func (s *Server) Run(ctx context.Context) error {
	loopCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	// Buffer of 3: the losing goroutines' sends never block after we
	// read the first result.
	errCh := make(chan error, 3)
	go func() { errCh <- s.public.Run(loopCtx) }()
	go func() { errCh <- s.control.Run(loopCtx) }()
	go func() { errCh <- s.api.Run(loopCtx) }()
	err := <-errCh
	cancel()
	return err
}
