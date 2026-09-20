//go:build integration

package httphandler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"

	httphandler "github.com/moxicom/cursed_matrix/back/internal/adapter/http-handler"
	"github.com/moxicom/cursed_matrix/back/internal/adapter/postgres"
	redisadapter "github.com/moxicom/cursed_matrix/back/internal/adapter/redis"
	"github.com/moxicom/cursed_matrix/back/internal/adapter/token"
	"github.com/moxicom/cursed_matrix/back/internal/app/achievement"
	"github.com/moxicom/cursed_matrix/back/internal/app/auth"
	"github.com/moxicom/cursed_matrix/back/internal/app/board"
	"github.com/moxicom/cursed_matrix/back/internal/app/graph"
	"github.com/moxicom/cursed_matrix/back/internal/app/profile"
	"github.com/moxicom/cursed_matrix/back/internal/domain/progression"
	"github.com/moxicom/cursed_matrix/back/internal/domain/shared"
)

// server builds the real stack — PostgreSQL, Redis, the service and the
// routes — so the flow under test is the one that runs in production.
func server(t *testing.T) http.Handler {
	t.Helper()
	// Generous on purpose: these tests exercise the flow, not the ceiling.
	return serverWithLimits(t, httphandler.RateLimits{
		AddressAttempts: 1000, AddressWindow: time.Minute,
		AccountAttempts: 1000, AccountWindow: time.Minute,
		ReadAttempts: 1000, ReadWindow: time.Minute,
	})
}

func serverWithLimits(t *testing.T, limits httphandler.RateLimits) http.Handler {
	t.Helper()

	dsn, redisURL := os.Getenv("TEST_DATABASE_URL"), os.Getenv("TEST_REDIS_URL")
	if dsn == "" || redisURL == "" {
		t.Skip("TEST_DATABASE_URL and TEST_REDIS_URL are not set")
	}
	quiet := slog.New(slog.NewTextHandler(io.Discard, nil))

	pool, err := postgres.NewPool(context.Background(), postgres.Options{
		URL: dsn, MaxConns: 4, MinConns: 1,
		MaxConnLifetime: time.Hour, MaxConnIdleTime: time.Minute, HealthCheckPeriod: time.Minute,
	}, quiet)
	if err != nil {
		t.Fatalf("pool: %v", err)
	}
	t.Cleanup(pool.Close)

	options, err := redis.ParseURL(redisURL)
	if err != nil {
		t.Fatalf("redis url: %v", err)
	}
	client := redis.NewClient(options)
	t.Cleanup(func() { _ = client.Close() })

	cached, err := redisadapter.NewCache(context.Background(), redisadapter.Options{
		URL:        redisURL,
		DefaultTTL: time.Minute,
	}, quiet)
	if err != nil {
		t.Fatalf("cache: %v", err)
	}
	t.Cleanup(func() { _ = cached.Close() })

	tokens := token.NewJWTIssuer("test-secret", 15*time.Minute, &shared.SystemClock{})
	service := auth.NewService(
		postgres.NewUserRepository(pool),
		redisadapter.NewRefreshStore(client),
		postgres.NewTxManager(pool, quiet),
		tokens,
		cached,
		&shared.SystemClock{},
		24*time.Hour,
	)
	awards := achievement.NewService(
		postgres.NewAchievementRepository(pool),
		postgres.NewUserRepository(pool),
		postgres.NewActivityRepository(pool),
		&shared.SystemClock{},
	)
	boards := board.NewService(board.Deps{
		Tasks:  postgres.NewTaskRepository(pool),
		Users:  postgres.NewUserRepository(pool),
		Tags:   postgres.NewTagRepository(pool),
		Links:  postgres.NewLinkRepository(pool),
		Tx:     postgres.NewTxManager(pool, quiet),
		Ledger: postgres.NewXPLedger(pool),
		Events: postgres.NewActivityRepository(pool),
		Awards: awards,
		XP:     progression.DefaultConfig(),
		Clock:  &shared.SystemClock{},
		Cache:  cached,
	})
	graphs := graph.NewService(
		postgres.NewLinkRepository(pool),
		postgres.NewTaskRepository(pool),
		postgres.NewUserRepository(pool),
		postgres.NewActivityRepository(pool),
		awards,
		postgres.NewTxManager(pool, quiet),
		&shared.SystemClock{},
	)
	limiter := redisadapter.NewRateLimiter(client, quiet)
	profiles := profile.NewService(
		postgres.NewUserRepository(pool),
		postgres.NewActivityRepository(pool),
		cached,
		postgres.NewTxManager(pool, quiet),
		awards,
		postgres.NewLeaderboardRepository(pool),
		&shared.SystemClock{},
	)
	return httphandler.Routes(
		httphandler.NewAPI(service, boards, graphs, profiles, awards, httphandler.NewCookieWriter(false), 24*time.Hour, limiter, limits, &shared.SystemClock{}),
		tokens, limiter, limits, profiles, &shared.SystemClock{}, quiet,
	)
}

type client struct {
	handler http.Handler
	cookies map[string]string
}

func (c *client) do(t *testing.T, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()

	request := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	request.Header.Set("Content-Type", "application/json")
	for name, value := range c.cookies {
		request.AddCookie(&http.Cookie{Name: name, Value: value})
	}
	if csrf, ok := c.cookies[httphandler.CSRFCookie]; ok {
		request.Header.Set(httphandler.CSRFHeader, csrf)
	}

	recorder := httptest.NewRecorder()
	c.handler.ServeHTTP(recorder, request)

	for _, cookie := range recorder.Result().Cookies() {
		// A browser drops a cookie whose expiry has passed, and drops one sent
		// with MaxAge<0 immediately. A jar that keeps both hides the case where
		// the session depends on a cookie the client no longer has.
		expired := cookie.MaxAge < 0 ||
			(!cookie.Expires.IsZero() && !cookie.Expires.After(time.Now()))
		if expired {
			delete(c.cookies, cookie.Name)
			continue
		}
		c.cookies[cookie.Name] = cookie.Value
	}
	return recorder
}

func newClient(t *testing.T) *client {
	return &client{handler: server(t), cookies: map[string]string{}}
}

func registerBody(username string) string {
	return `{"username":"` + username + `","password":"correct horse battery","timezone":"Europe/Moscow"}`
}

func TestRegistrationIssuesCookiesThePageCannotRead(t *testing.T) {
	c := newClient(t)
	response := c.do(t, http.MethodPost, "/auth/register", registerBody("t_"+uuid.NewString()[:8]))

	if response.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201: %s", response.Code, response.Body)
	}

	tests := []struct {
		name         string
		cookie       string
		wantHTTPOnly bool
		wantSameSite http.SameSite
		wantPath     string
	}{
		{name: "access", cookie: httphandler.AccessCookie, wantHTTPOnly: true, wantSameSite: http.SameSiteLaxMode, wantPath: "/api"},
		{name: "refresh", cookie: httphandler.RefreshCookie, wantHTTPOnly: true, wantSameSite: http.SameSiteStrictMode, wantPath: "/api/v1/auth"},
		{name: "csrf", cookie: httphandler.CSRFCookie, wantHTTPOnly: false, wantSameSite: http.SameSiteLaxMode, wantPath: "/"},
	}

	issued := map[string]*http.Cookie{}
	for _, cookie := range response.Result().Cookies() {
		issued[cookie.Name] = cookie
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cookie, ok := issued[tc.cookie]
			if !ok {
				t.Fatalf("%s was not set", tc.cookie)
			}
			if cookie.HttpOnly != tc.wantHTTPOnly {
				t.Errorf("HttpOnly = %v, want %v", cookie.HttpOnly, tc.wantHTTPOnly)
			}
			if cookie.SameSite != tc.wantSameSite {
				t.Errorf("SameSite = %v, want %v", cookie.SameSite, tc.wantSameSite)
			}
			if cookie.Path != tc.wantPath {
				t.Errorf("Path = %q, want %q", cookie.Path, tc.wantPath)
			}
		})
	}

	t.Run("no token is in the body", func(t *testing.T) {
		body := response.Body.String()
		for _, token := range []string{issued[httphandler.AccessCookie].Value, issued[httphandler.RefreshCookie].Value} {
			if bytes.Contains([]byte(body), []byte(token)) {
				t.Error("a token was returned in the response body")
			}
		}
	})
}

func TestSessionLifecycle(t *testing.T) {
	c := newClient(t)
	username := "t_" + uuid.NewString()[:8]

	if got := c.do(t, http.MethodPost, "/auth/register", registerBody(username)).Code; got != http.StatusCreated {
		t.Fatalf("register: %d", got)
	}

	tests := []struct {
		name       string
		method     string
		path       string
		body       string
		dropCSRF   bool
		wantStatus int
	}{
		{name: "the account reads itself", method: http.MethodGet, path: "/me", wantStatus: http.StatusOK},
		{name: "a write without the csrf header is refused", method: http.MethodPatch, path: "/me", body: `{"showInLeaderboard":true}`, dropCSRF: true, wantStatus: http.StatusForbidden},
		{name: "a write with it succeeds", method: http.MethodPatch, path: "/me", body: `{"showInLeaderboard":true,"language":"RU"}`, wantStatus: http.StatusOK},
		{name: "refreshing rotates the pair", method: http.MethodPost, path: "/auth/refresh", wantStatus: http.StatusNoContent},
		{name: "logging out ends it", method: http.MethodPost, path: "/auth/logout", wantStatus: http.StatusNoContent},
		{name: "and the session is gone", method: http.MethodGet, path: "/me", wantStatus: http.StatusUnauthorized},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			saved := c.cookies[httphandler.CSRFCookie]
			if tc.dropCSRF {
				delete(c.cookies, httphandler.CSRFCookie)
			}

			response := c.do(t, tc.method, tc.path, tc.body)
			if tc.dropCSRF {
				c.cookies[httphandler.CSRFCookie] = saved
			}

			if response.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d: %s", response.Code, tc.wantStatus, response.Body)
			}
		})
	}
}

func TestReplayedRefreshTokenRevokesTheFamily(t *testing.T) {
	c := newClient(t)
	if got := c.do(t, http.MethodPost, "/auth/register", registerBody("t_"+uuid.NewString()[:8])).Code; got != http.StatusCreated {
		t.Fatalf("register: %d", got)
	}

	stolen := c.cookies[httphandler.RefreshCookie]
	csrf := c.cookies[httphandler.CSRFCookie]
	access := c.cookies[httphandler.AccessCookie]
	if got := c.do(t, http.MethodPost, "/auth/refresh", "").Code; got != http.StatusNoContent {
		t.Fatalf("first refresh: %d", got)
	}
	rotated := c.cookies[httphandler.RefreshCookie]

	c.cookies[httphandler.RefreshCookie] = stolen
	if got := c.do(t, http.MethodPost, "/auth/refresh", "").Code; got != http.StatusUnauthorized {
		t.Fatalf("replaying the consumed token = %d, want 401", got)
	}

	// The replay cleared this client's cookies, but the honest holder of the
	// rotated token never saw that response — so restore what it would still
	// be carrying, and check the token is dead anyway. That is the point: one
	// of the two is an attacker and there is no way to tell which.
	c.cookies[httphandler.RefreshCookie] = rotated
	c.cookies[httphandler.CSRFCookie] = csrf
	c.cookies[httphandler.AccessCookie] = access
	if got := c.do(t, http.MethodPost, "/auth/refresh", "").Code; got != http.StatusUnauthorized {
		t.Fatalf("the rotated token still works after a replay: %d", got)
	}
}

func TestLoginRefusesUnknownAccountsAndWrongPasswords(t *testing.T) {
	c := newClient(t)
	username := "t_" + uuid.NewString()[:8]
	if got := c.do(t, http.MethodPost, "/auth/register", registerBody(username)).Code; got != http.StatusCreated {
		t.Fatalf("register: %d", got)
	}

	tests := []struct {
		name       string
		body       string
		wantStatus int
		wantCode   shared.ErrorCode
	}{
		{name: "the right password", body: registerBody(username), wantStatus: http.StatusOK},
		{
			name:       "a wrong password",
			body:       `{"username":"` + username + `","password":"not the password"}`,
			wantStatus: http.StatusUnauthorized, wantCode: shared.CodeInvalidCredentials,
		},
		{
			name:       "an account that does not exist",
			body:       `{"username":"absent_` + uuid.NewString()[:8] + `","password":"correct horse battery"}`,
			wantStatus: http.StatusUnauthorized, wantCode: shared.CodeInvalidCredentials,
		},
		{
			name:       "a username that is too short to exist",
			body:       `{"username":"ab","password":"correct horse battery"}`,
			wantStatus: http.StatusUnauthorized, wantCode: shared.CodeInvalidCredentials,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			fresh := newClient(t)
			response := fresh.do(t, http.MethodPost, "/auth/login", tc.body)

			if response.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d: %s", response.Code, tc.wantStatus, response.Body)
			}
			if tc.wantCode == "" {
				return
			}

			var body struct {
				Error struct {
					Code shared.ErrorCode `json:"code"`
				} `json:"error"`
			}
			if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
				t.Fatalf("decode: %v", err)
			}
			if body.Error.Code != tc.wantCode {
				t.Errorf("code = %q, want %q — a different code would say whether the account exists",
					body.Error.Code, tc.wantCode)
			}
		})
	}
}

func TestRegistrationRejectsWeakInput(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{name: "username too short", body: `{"username":"ab","password":"correct horse battery","timezone":"UTC"}`},
		{name: "username with a space", body: `{"username":"two words","password":"correct horse battery","timezone":"UTC"}`},
		{name: "password too short", body: `{"username":"someone_new","password":"short","timezone":"UTC"}`},
		{name: "unknown field", body: `{"username":"someone_new","password":"correct horse battery","timezone":"UTC","admin":true}`},
		{name: "not json", body: `nonsense`},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := newClient(t)
			if got := c.do(t, http.MethodPost, "/auth/register", tc.body).Code; got != http.StatusUnprocessableEntity {
				t.Errorf("status = %d, want 422", got)
			}
		})
	}
}

// The access cookie expires in fifteen minutes and the refresh cookie lives for
// thirty days. A session that cannot be refreshed, or logged out, once the
// short one is gone would make the long one pointless — and would leave a
// revoked-looking session alive in the store.
func TestRefreshAndLogoutSurviveTheAccessCookie(t *testing.T) {
	tests := []struct {
		name       string
		call       string
		wantStatus int
	}{
		{name: "refreshing", call: "/auth/refresh", wantStatus: http.StatusNoContent},
		{name: "logging out", call: "/auth/logout", wantStatus: http.StatusNoContent},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := newClient(t)
			if got := c.do(t, http.MethodPost, "/auth/register", registerBody("t_"+uuid.NewString()[:8])).Code; got != http.StatusCreated {
				t.Fatalf("register: %d", got)
			}

			refresh := c.cookies[httphandler.RefreshCookie]
			// The browser has dropped the access cookie; the other two remain.
			delete(c.cookies, httphandler.AccessCookie)

			if got := c.do(t, http.MethodPost, tc.call, "").Code; got != tc.wantStatus {
				t.Fatalf("%s without the access cookie = %d, want %d", tc.call, got, tc.wantStatus)
			}

			// Whatever the call was, the token it consumed must be dead.
			c.cookies[httphandler.RefreshCookie] = refresh
			if got := c.do(t, http.MethodPost, "/auth/refresh", "").Code; got == http.StatusNoContent {
				t.Error("the consumed refresh token still works")
			}
		})
	}
}

func TestDecodeRequiresJSON(t *testing.T) {
	tests := []struct {
		name        string
		contentType string
		wantStatus  int
	}{
		{name: "json is accepted", contentType: "application/json", wantStatus: http.StatusCreated},
		{name: "json with a charset is accepted", contentType: "application/json; charset=utf-8", wantStatus: http.StatusCreated},
		{name: "a form post is refused", contentType: "application/x-www-form-urlencoded", wantStatus: http.StatusUnprocessableEntity},
		{name: "text/plain is refused", contentType: "text/plain", wantStatus: http.StatusUnprocessableEntity},
		{name: "no content type is refused", contentType: "", wantStatus: http.StatusUnprocessableEntity},
	}

	handler := server(t)
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			body := registerBody("t_" + uuid.NewString()[:8])
			request := httptest.NewRequest(http.MethodPost, "/auth/register", bytes.NewBufferString(body))
			if tc.contentType != "" {
				request.Header.Set("Content-Type", tc.contentType)
			} else {
				request.Header.Del("Content-Type")
			}

			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, request)

			if recorder.Code != tc.wantStatus {
				t.Errorf("status = %d, want %d: %s", recorder.Code, tc.wantStatus, recorder.Body)
			}
		})
	}
}

func post(t *testing.T, handler http.Handler, address, path, body string) *httptest.ResponseRecorder {
	t.Helper()

	request := httptest.NewRequest(http.MethodPost, path, bytes.NewBufferString(body))
	request.Header.Set("Content-Type", "application/json")
	// The address the reverse proxy reports, which is what the per-address
	// ceiling counts against.
	request.Header.Set("X-Real-IP", address)

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	return recorder
}

func errorCode(t *testing.T, recorder *httptest.ResponseRecorder) shared.ErrorCode {
	t.Helper()

	var body struct {
		Error struct {
			Code shared.ErrorCode `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode %q: %v", recorder.Body.String(), err)
	}
	return body.Error.Code
}
