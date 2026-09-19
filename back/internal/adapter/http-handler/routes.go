package httphandler

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	gen "github.com/moxicom/cursed_matrix/back/internal/adapter/http-handler/gen"
	"github.com/moxicom/cursed_matrix/back/internal/app/port"
	"github.com/moxicom/cursed_matrix/back/internal/domain/shared"
)

// Routes assembles the API under one mount point.
//
// Three groups, and the difference between them is the whole access policy:
// registering and signing in are open, refreshing and logging out need the
// refresh cookie but tolerate a stale access token, and everything else needs a
// valid session. The last two check the CSRF header on every unsafe method.
func Routes(api *API, tokens port.TokenIssuer, limiter port.RateLimiter, limits RateLimits) http.Handler {
	router := chi.NewRouter()

	// No CSRF check on the way in: the client has no token to echo before its
	// first response, and there is no session to protect yet. SameSite=Lax is
	// what keeps a cross-site POST from carrying cookies here.
	router.Group(func(public chi.Router) {
		public.Use(LimitByAddress(limiter, limits))
		public.Post("/auth/register", api.Register)
		public.Post("/auth/login", api.Login)
	})

	// Refreshing and logging out are authenticated by the refresh cookie itself,
	// not by the access token: that one is expected to be stale or gone.
	router.Group(func(refreshing chi.Router) {
		refreshing.Use(RequireCSRF)
		refreshing.Post("/auth/refresh", api.Refresh)
		refreshing.Post("/auth/logout", api.Logout)
	})

	router.Group(func(session chi.Router) {
		session.Use(RequireCSRF)
		session.Use(Authenticate(tokens))
		session.Post("/auth/logout-all", api.LogoutAll)
		session.Get("/me", api.GetMe)
		session.Patch("/me", api.UpdateSettings)

		// Only the board carries the read ceiling. On the whole group it would
		// mean a client stuck in a loop also loses "sign out everywhere" —
		// throttling the one endpoint that stops the loop.
		session.With(LimitReads(limiter, limits)).Get("/tasks", bind(api).ListTasks)
		session.Post("/tasks", api.CreateTask)
		session.Patch("/tasks/{taskId}", bind(api).UpdateTask)
		session.Delete("/tasks/{taskId}", bind(api).DeleteTask)
		session.Post("/tasks/{taskId}/subtasks", bind(api).CreateSubtask)
	})

	return router
}

// bind is the generated query-string decoder. Its error handler is ours, so a
// malformed parameter answers in the same envelope as every other refusal
// instead of the plain text the generator would write.
func bind(api *API) *gen.ServerInterfaceWrapper {
	return &gen.ServerInterfaceWrapper{
		Handler: api,
		ErrorHandlerFunc: func(w http.ResponseWriter, r *http.Request, err error) {
			WriteError(w, r, shared.WrapError(err, shared.CodeValidationFailed,
				map[string]any{"reason": "malformed query"}))
		},
	}
}

// compile-time proof that every operation of the contract is routed
var _ = gen.ServerInterface(nil)
