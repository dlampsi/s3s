package server

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"s3s/info"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
)

var log *zap.SugaredLogger

type Server struct {
	logger          *zap.Logger
	interruptChanel chan os.Signal
	srv             *http.Server
	reg             *prometheus.Registry
}

type ServerOption func(*Server)

func WithLogger(l *zap.Logger) ServerOption {
	return func(s *Server) { s.logger = l }
}

func WithRegistry(r *prometheus.Registry) ServerOption {
	return func(s *Server) { s.reg = r }
}

func NewServer(addr string, opts ...ServerOption) *Server {
	srv := &Server{
		logger: zap.NewNop(),
		srv: &http.Server{
			Addr: addr,
		},
	}

	for _, opt := range opts {
		opt(srv)
	}

	log = srv.logger.Sugar().Named("server")

	http.Handle("/version", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data := []byte(fmt.Sprintf(
			`{"version": "%s", "build_time": "%s"}`,
			info.Version, info.BuildTime,
		))
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Length", strconv.Itoa(len(data)))
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write(data); err != nil {
			fmt.Fprintln(os.Stderr, "can't write data: ", err)
		}
	}))

	// http.Handle("/health",)

	http.Handle("/metrics", promhttp.HandlerFor(srv.reg, promhttp.HandlerOpts{}))

	return srv
}

// Serve rest api service.
func (s *Server) Serve() {
	// Start rest api service
	shutdown := make(chan error, 2)
	go func() {
		log.Infof("Binding on %s", s.srv.Addr)
		if err := s.srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			shutdown <- err
		}
	}()

	// Graceful shutdown
	s.interruptChanel = make(chan os.Signal, 2)
	signal.Notify(s.interruptChanel, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)

	select {
	case x := <-s.interruptChanel:
		log.Info("Recived signal: ", x.String())
	case err := <-shutdown:
		log.Error(err.Error())
	}

	timeout, cancelFunc := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelFunc()

	if err := s.srv.Shutdown(timeout); err != nil {
		log.Errorw("Server stopped with error", "err", err.Error())
	}
}
