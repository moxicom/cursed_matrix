package metrics_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/moxicom/cursed_matrix/back/internal/adapter/metrics"
)

// TestMiddlewareLabelsByRoutePattern is the reason the label is the pattern
// and not the path: one counter per task id would be a new time series per
// task, which is how a metrics store is destroyed.
func TestMiddlewareLabelsByRoutePattern(t *testing.T) {
	tests := []struct {
		name      string
		mount     string
		request   string
		status    int
		wantRoute string
	}{
		{
			name:      "a parameterised route reports its pattern",
			mount:     "/tasks/{taskId}",
			request:   "/tasks/11111111-1111-1111-1111-111111111111",
			status:    http.StatusOK,
			wantRoute: "/tasks/{taskId}",
		},
		{
			name:      "a static route reports itself",
			mount:     "/tasks",
			request:   "/tasks",
			status:    http.StatusCreated,
			wantRoute: "/tasks",
		},
		{
			// Nothing matched, so there is no pattern to name; a path would
			// be an unbounded label.
			name:      "a request that matched nothing is not a series of its own",
			mount:     "/tasks",
			request:   "/nowhere",
			status:    http.StatusNotFound,
			wantRoute: "unmatched",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			registry := metrics.New()
			router := chi.NewRouter()
			router.Use(registry.Middleware)
			router.Get(tt.mount, func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.status)
			})

			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, tt.request, nil))

			exposed := scrape(t, registry)
			want := `http_requests_total{method="GET",route="` + tt.wantRoute + `"`
			if !strings.Contains(exposed, want) {
				t.Errorf("no counter for %s\n%s", tt.wantRoute, exposed)
			}
			if !strings.Contains(exposed, `http_request_duration_seconds_count{method="GET",route="`+tt.wantRoute+`"}`) {
				t.Errorf("no duration for %s", tt.wantRoute)
			}
		})
	}
}

func TestRecordCache(t *testing.T) {
	registry := metrics.New()

	registry.RecordCache("user", "hit")
	registry.RecordCache("user", "hit")
	registry.RecordCache("user", "miss")

	exposed := scrape(t, registry)
	for _, want := range []string{
		`cache_events_total{event="hit",kind="user"} 2`,
		`cache_events_total{event="miss",kind="user"} 1`,
	} {
		if !strings.Contains(exposed, want) {
			t.Errorf("missing %q\n%s", want, exposed)
		}
	}
}

// TestRuntimeCollectorsAreRegistered guards the thing nobody notices is gone
// until an incident: the process and runtime series the dashboards are built
// on.
func TestRuntimeCollectorsAreRegistered(t *testing.T) {
	exposed := scrape(t, metrics.New())

	for _, want := range []string{"go_goroutines", "go_memstats_alloc_bytes"} {
		if !strings.Contains(exposed, want) {
			t.Errorf("%s is not exposed", want)
		}
	}
}

func scrape(t *testing.T, registry *metrics.Registry) string {
	t.Helper()

	recorder := httptest.NewRecorder()
	handler := promhttp.HandlerFor(registry.Registry, promhttp.HandlerOpts{})
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/metrics", nil))

	if recorder.Code != http.StatusOK {
		t.Fatalf("scrape = %d", recorder.Code)
	}
	return recorder.Body.String()
}
