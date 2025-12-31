package collector

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"path/filepath"
	"time"

	"github.com/zmack/otis/config"
)

type Server struct {
	config        *config.Config
	httpServer    *http.Server
	traceHandler  *TraceHandler
	metricsHandler *MetricsHandler
	logsHandler   *LogsHandler
}

func NewServer(cfg *config.Config) (*Server, error) {
	traceWriter, err := NewFileWriter(filepath.Join(cfg.OutputDir, cfg.TraceFileName))
	if err != nil {
		return nil, fmt.Errorf("failed to create trace writer: %w", err)
	}

	metricsWriter, err := NewFileWriter(filepath.Join(cfg.OutputDir, cfg.MetricFileName))
	if err != nil {
		return nil, fmt.Errorf("failed to create metrics writer: %w", err)
	}

	logsWriter, err := NewFileWriter(filepath.Join(cfg.OutputDir, cfg.LogFileName))
	if err != nil {
		return nil, fmt.Errorf("failed to create logs writer: %w", err)
	}

	traceHandler := NewTraceHandler(traceWriter)
	metricsHandler := NewMetricsHandler(metricsWriter)
	logsHandler := NewLogsHandler(logsWriter)

	mux := http.NewServeMux()
	mux.Handle("/v1/traces", traceHandler)
	mux.Handle("/v1/metrics", metricsHandler)
	mux.Handle("/v1/logs", logsHandler)

	httpServer := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.ServerPort),
		Handler:      loggingMiddleware(mux),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	return &Server{
		config:         cfg,
		httpServer:     httpServer,
		traceHandler:   traceHandler,
		metricsHandler: metricsHandler,
		logsHandler:    logsHandler,
	}, nil
}

func (s *Server) Start() error {
	log.Printf("Starting OTLP collector on port %d", s.config.ServerPort)
	log.Printf("Trace endpoint: http://localhost:%d/v1/traces", s.config.ServerPort)
	log.Printf("Metrics endpoint: http://localhost:%d/v1/metrics", s.config.ServerPort)
	log.Printf("Logs endpoint: http://localhost:%d/v1/logs", s.config.ServerPort)
	log.Printf("Output directory: %s", s.config.OutputDir)

	if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("failed to start server: %w", err)
	}
	return nil
}

func (s *Server) Shutdown(ctx context.Context) error {
	log.Println("Shutting down server...")
	return s.httpServer.Shutdown(ctx)
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip logging HTTP/2 connection preface
		if r.Method == "PRI" {
			next.ServeHTTP(w, r)
			return
		}

		start := time.Now()
		log.Printf("Started %s %s", r.Method, r.URL.Path)
		next.ServeHTTP(w, r)
		log.Printf("Completed %s %s in %v", r.Method, r.URL.Path, time.Since(start))
	})
}
