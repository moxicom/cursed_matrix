package metrics

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

func chiContext(req *http.Request) string {
	routeCtx := chi.RouteContext(req.Context())
	if routeCtx == nil {
		return ""
	}
	return routeCtx.RoutePattern()
}
