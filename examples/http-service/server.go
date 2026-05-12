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

func NewServer(port int, cancel context.CancelFunc, logger *slog.Logger) *server {

	mux := http.NewServeMux()

	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		// reqLoggerMiddleware is outermost so request fields are available to authMiddleware.
			Handler: reqLoggerMiddleware(logger)(authMiddleware()(mux)),
	}

	s := &server{
		httpServer:  srv,
		cancel:      cancel,
		logger:      logger,
		userService: NewUserService(logger),
	}

	// Define routes and handlers for the HTTP server.
	mux.HandleFunc("/users", s.handleUsers)

	return s

}

func (s *server) Start() error {
	ln, err := net.Listen("tcp", s.httpServer.Addr)
	if err != nil {
		return errx.Wrap(err, "failed to start server", "component", "http_server")
	}

	if tcpAddr, ok := ln.Addr().(*net.TCPAddr); ok {
		port := tcpAddr.Port
		s.logger.Info("http server started", slog.Int("port", port))
	} else {
		s.logger.Info("http server started", slog.String("address", ln.Addr().String()))
	}

	if err := s.httpServer.Serve(ln); !errors.Is(err, http.ErrServerClosed) {
		s.logger.Error("http server failed", slog.Any("error", err))
		return errx.Wrap(err, "http server failed", "component", "http_server")
	}

	return nil
}

func (s *server) Shutdown(ctx context.Context) error {
	if err := s.httpServer.Shutdown(ctx); err != nil {
		s.logger.Error("http server shutdown failed", slog.Any("error", err))
		return errx.Wrap(err, "http server shutdown failed", "component", "http_server")
	}
	return nil
}
