package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"

	"github.com/DaniDeer/go-logx/errx"
)

type server struct {
	httpServer  *http.Server
	cancel      context.CancelFunc
	logger      *slog.Logger
	userService *UserService
}

// NewServer wires the middleware chain and registers routes.
// requestLogger is outermost so auth and handler logs are captured in the
// consolidated request log line it emits after the chain returns.
func NewServer(port *int, cancel context.CancelFunc, logger *slog.Logger) *server {
	mux := http.NewServeMux()

	srv := &http.Server{
		Addr: fmt.Sprintf(":%d", *port),
		// requestIDMiddleware is outermost so the ID is available on all paths including auth rejections.
		Handler: requestIDMiddleware()(requestLogger(logger)(authMiddleware()(mux))),
	}

	s := &server{
		httpServer:  srv,
		cancel:      cancel,
		logger:      logger.With(slog.String("component", "http_server")),
		userService: NewUserService(logger),
	}

	mux.HandleFunc("GET /users", s.handleUsers)

	return s
}

func (s *server) Start() error {
	ln, err := net.Listen("tcp", s.httpServer.Addr)
	if err != nil {
		return errx.Wrap(err, "failed to start server", "component", "http_server")
	}

	if tcpAddr, ok := ln.Addr().(*net.TCPAddr); ok {
		s.logger.Info("http server started", slog.Int("port", tcpAddr.Port))
	} else {
		s.logger.Info("http server started", slog.String("address", ln.Addr().String()))
	}

	if err := s.httpServer.Serve(ln); !errors.Is(err, http.ErrServerClosed) {
		return errx.Wrap(err, "http server failed", "component", "http_server")
	}

	return nil
}

func (s *server) Shutdown(ctx context.Context) error {
	if err := s.httpServer.Shutdown(ctx); err != nil {
		return errx.Wrap(err, "http server shutdown failed", "component", "http_server")
	}
	return nil
}
