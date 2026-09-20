package httphandler

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	gen "github.com/moxicom/cursed_matrix/back/internal/adapter/http-handler/gen"
	"github.com/moxicom/cursed_matrix/back/internal/app/port"
	"github.com/moxicom/cursed_matrix/back/internal/app/profile"
	"github.com/moxicom/cursed_matrix/back/internal/domain/shared"
)

// Routes assembles the API under one mount point.
//
// Three groups, and the difference between them is the whole access policy:
// registering and signing in are open, refreshing and logging out need the
// refresh cookie but tolerate a stale access token, and everything else needs a
// valid session. The last two check the CSRF header on every unsafe method.
func Routes(
	api *API,
	tokens port.TokenIssuer,
	limiter port.RateLimiter,
	limits RateLimits,
	profiles *profile.Service,
	clock shared.Clock,
	log *slog.Logger,
) http.Handler {
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
		// After authentication, so it knows whose day it is, and before the
		// handlers, so any visit counts and not only a board read.
		session.Use(TrackStreak(profiles, log))

		// The account itself is always reachable: someone whose plan lapsed
		// still has to be able to see it, change their settings and pay.
		session.Post("/auth/logout-all", api.LogoutAll)
		session.Get("/me", api.GetMe)
		session.Patch("/me", api.UpdateSettings)

		// Everything below is the application itself, and an account whose
		// paid time ran out cannot reach it. chi fixes a group's middleware
		// before its routes, so the gate needs a group of its own.
		session.Group(func(app chi.Router) {
			app.Use(RequireSubscription(clock))

			// Only the board carries the read ceiling. On the whole group it would
			// mean a client stuck in a loop also loses "sign out everywhere" —
			// throttling the one endpoint that stops the loop.
			app.With(LimitReads(limiter, limits)).Get("/tasks", bind(api).ListTasks)
			app.Post("/tasks", api.CreateTask)
			app.Patch("/tasks/{taskId}", bind(api).UpdateTask)
			app.Delete("/tasks/{taskId}", bind(api).DeleteTask)
			app.Post("/tasks/{taskId}/subtasks", bind(api).CreateSubtask)
			app.Post("/tasks/{taskId}/complete", bind(api).CompleteTask)
			app.Post("/tasks/{taskId}/reopen", bind(api).ReopenTask)
			app.Post("/tasks/{taskId}/move", bind(api).MoveTask)
			app.Post("/tasks/{taskId}/promote", bind(api).PromoteTask)

			app.Get("/tags", api.ListTags)
			app.Post("/tasks/{taskId}/tags", bind(api).AttachTag)
			app.Delete("/tasks/{taskId}/tags/{tagId}", bind(api).DetachTag)

			app.Get("/achievements", api.ListAchievements)
			app.Get("/leaderboard", bind(api).Leaderboard)
			app.With(LimitReads(limiter, limits)).Get("/graph", bind(api).GetGraph)
			// Search and the graph are unbounded work per call, like the
			// board: two ILIKE scans with a tag subquery per row, and a
			// snapshot of the whole network. The contract asks for search to
			// be throttled by name.
			app.With(LimitReads(limiter, limits)).Get("/search", bind(api).Search)
			app.Get("/activity/heatmap", api.ActivityHeatmap)
			app.Get("/activity/events", bind(api).ActivityEvents)
			app.Get("/activity/stats", api.ActivityStats)
			app.Post("/activity/graph-opened", api.GraphOpened)

			app.Post("/links", api.CreateLink)
			app.Patch("/links/{linkId}", bind(api).UpdateLink)
			app.Delete("/links/{linkId}", bind(api).DeleteLink)
		})
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
