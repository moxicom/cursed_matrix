package httpserver_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	httpserver "github.com/moxicom/cursed_matrix/back/internal/adapter/http-server"
	"github.com/moxicom/cursed_matrix/back/internal/adapter/metrics"
	"github.com/moxicom/cursed_matrix/back/internal/config"
	"github.com/moxicom/cursed_matrix/back/pkg/utils"
)

func quiet() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func settings() *config.Config {
	return &config.Config{
		HTTP: config.HTTPConfig{
			Port:              8080,
			RequestTimeout:    2 * time.Second,
			ReadHeaderTimeout: time.Second,
			ShutdownTimeout:   time.Second,
		},
	}
}

func TestRouterOperationalEndpoints(t *testing.T) {
	tests := []struct {
		name       string
		path       string
		ready      func(context.Context) error
		wantStatus int
		wantBody   string
	}{
		{
			// Liveness says the process is running, which is a different
			// question from whether its dependencies are: it must not fail
			// because the database is briefly away, or an orchestrator would
			// restart a server that is merely waiting.
			name:       "liveness answers without asking anything",
			path:       "/healthz",
			ready:      func(context.Context) error { return errors.New("database is away") },
			wantStatus: http.StatusOK,
			wantBody:   "ok",
		},
		{
			name:       "readiness answers when the dependencies do",
			path:       "/readyz",
			ready:      func(context.Context) error { return nil },
			wantStatus: http.StatusOK,
			wantBody:   "ready",
		},
		{
			name:       "and refuses when they do not",
			path:       "/readyz",
			ready:      func(context.Context) error { return errors.New("database is away") },
			wantStatus: http.StatusServiceUnavailable,
		},
		{
			name:       "metrics are exposed",
			path:       "/metrics",
			ready:      func(context.Context) error { return nil },
			wantStatus: http.StatusOK,
			wantBody:   "go_goroutines",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := httpserver.NewRouter(settings(), quiet(), metrics.New().Registry, tt.ready, nil)

			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, tt.path, nil))

			if recorder.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", recorder.Code, tt.wantStatus)
			}
			if tt.wantBody != "" && !strings.Contains(recorder.Body.String(), tt.wantBody) {
				t.Errorf("body = %q, want it to mention %q", recorder.Body.String(), tt.wantBody)
			}
		})
	}
}

func TestRouterMountsTheAPI(t *testing.T) {
	// A chi router, like the real API: chi's Mount hands a nested chi router
	// the path with the prefix taken off, and leaves it alone for anything
	// else.
	api := chi.NewRouter()
	api.Get("/ping", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	})

	tests := []struct {
		name       string
		api        http.Handler
		path       string
		wantStatus int
	}{
		{name: "mounted under the version", api: api, path: "/api/v1/ping", wantStatus: http.StatusTeapot},
		{name: "and not at the root", api: api, path: "/ping", wantStatus: http.StatusNotFound},
		{
			// The API is nil until Redis is reachable; the operational
			// endpoints still have to answer so the deployment can be seen.
			name:       "absent when nothing is wired",
			api:        nil,
			path:       "/api/v1/ping",
			wantStatus: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := httpserver.NewRouter(settings(), quiet(), metrics.New().Registry,
				func(context.Context) error { return nil }, tt.api)

			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, tt.path, nil))

			if recorder.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", recorder.Code, tt.wantStatus)
			}
		})
	}
}

// TestRequestCarriesItsIdentity is what makes a support request answerable:
// the id in the error the user quotes has to be the one in the log line.
func TestRequestCarriesItsIdentity(t *testing.T) {
	var (
		seenID     string
		seenLogger bool
	)
	api := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenID = utils.RequestID(r.Context())
		seenLogger = utils.LoggerFromContext(r.Context()) != slog.Default()
		w.WriteHeader(http.StatusOK)
	})

	router := httpserver.NewRouter(settings(), quiet(), metrics.New().Registry,
		func(context.Context) error { return nil }, api)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/anything", nil))

	if seenID == "" {
		t.Error("the handler saw no request id")
	}
	if !seenLogger {
		t.Error("the handler was given the default logger, not the request's own")
	}
}

// TestPanicDoesNotTakeTheServerDown: one handler's bug must cost that request
// and nothing else.
func TestPanicDoesNotTakeTheServerDown(t *testing.T) {
	api := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("a bug in a handler")
	})

	router := httpserver.NewRouter(settings(), quiet(), metrics.New().Registry,
		func(context.Context) error { return nil }, api)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/boom", nil))

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", recorder.Code)
	}

	// The next request is served as if nothing happened.
	recorder = httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if recorder.Code != http.StatusOK {
		t.Errorf("the server did not survive: %d", recorder.Code)
	}
}

func TestExtraMiddlewareRuns(t *testing.T) {
	var ran bool
	mark := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ran = true
			next.ServeHTTP(w, r)
		})
	}

	// Passed in rather than added later because chi refuses Use after the
	// first route, which is the whole reason the parameter exists.
	router := httpserver.NewRouter(settings(), quiet(), metrics.New().Registry,
		func(context.Context) error { return nil }, nil, mark)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/healthz", nil))

	if !ran {
		t.Error("the middleware was not run")
	}
}

func TestShutdownStopsServing(t *testing.T) {
	cfg := settings()
	cfg.HTTP.Port = 0

	server := httpserver.NewServer(cfg, http.NewServeMux(), quiet())

	serving := make(chan error, 1)
	go func() { serving <- server.Serve() }()

	if err := server.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown: %v", err)
	}

	select {
	case err := <-serving:
		// A closed server is how Serve ends; it is not a failure.
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			t.Errorf("Serve: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Serve did not return after the shutdown")
	}
}
