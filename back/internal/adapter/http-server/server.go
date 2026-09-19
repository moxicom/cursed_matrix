package httpserver

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/moxicom/cursed_matrix/back/internal/config"
	"github.com/moxicom/cursed_matrix/back/pkg/utils"
)

type Server struct {
	http *http.Server
	log  *slog.Logger
}

// NewRouter takes extra middleware because chi refuses Use() after the first route.
func NewRouter(
	cfg *config.Config,
	log *slog.Logger,
	registry *prometheus.Registry,
	ready func(context.Context) error,
	extra ...func(http.Handler) http.Handler,
) *chi.Mux {
	router := chi.NewRouter()

	router.Use(middleware.RequestID)
	router.Use(requestContext(log))
	router.Use(middleware.Recoverer)
	router.Use(middleware.Timeout(cfg.HTTP.RequestTimeout))
	for _, mw := range extra {
		router.Use(mw)
	}

	router.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	router.Get("/readyz", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := ready(ctx); err != nil {
			utils.LoggerFromContext(ctx).WarnContext(ctx, "readiness check failed", "err", err)
			http.Error(w, "not ready", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ready"))
	})

	router.Handle("/metrics", promhttp.HandlerFor(registry, promhttp.HandlerOpts{}))

	return router
}

func requestContext(log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := utils.WithRequestID(r.Context(), middleware.GetReqID(r.Context()))
			ctx = utils.ContextWithLogger(ctx, log.With("method", r.Method, "path", r.URL.Path))

			wrapped := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
			started := time.Now()
			next.ServeHTTP(wrapped, r.WithContext(ctx))

			utils.LoggerFromContext(ctx).InfoContext(ctx, "request",
				"status", wrapped.Status(),
				"bytes", wrapped.BytesWritten(),
				"durationMs", time.Since(started).Milliseconds(),
			)
		})
	}
}

func NewServer(cfg *config.Config, handler http.Handler, log *slog.Logger) *Server {
	return &Server{
		http: &http.Server{
			Addr:              cfg.Addr(),
			Handler:           handler,
			ReadHeaderTimeout: cfg.HTTP.ReadHeaderTimeout,
			ReadTimeout:       cfg.HTTP.ReadTimeout,
			WriteTimeout:      cfg.HTTP.WriteTimeout,
			IdleTimeout:       cfg.HTTP.IdleTimeout,
		},
		log: log,
	}
}

// Serve blocks until the listener stops. A closed server is not an error.
func (s *Server) Serve() error {
	s.log.Info("http server listening", "addr", s.http.Addr)

	if err := s.http.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

// Shutdown stops accepting connections and waits for in-flight requests, or
// until ctx expires. The process owns the deadline, not the server.
func (s *Server) Shutdown(ctx context.Context) error {
	return s.http.Shutdown(ctx)
}
