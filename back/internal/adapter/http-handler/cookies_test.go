package httphandler_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	httphandler "github.com/moxicom/cursed_matrix/back/internal/adapter/http-handler"
)

// The CSRF cookie must outlive the access cookie. Refreshing is an unsafe
// request behind the CSRF check: a token that expired with the access cookie
// leaves the client nothing to echo, the refresh is refused, and the session
// ends after one access lifetime however long the refresh cookie lasts.
func TestIssuedCookieLifetimes(t *testing.T) {
	const (
		accessTTL  = 15 * time.Minute
		refreshTTL = 720 * time.Hour
		// Slack for the clock moving between Issue and the assertions.
		tolerance = 5 * time.Second
	)

	issued := time.Now()
	recorder := httptest.NewRecorder()
	writer := httphandler.NewCookieWriter(true)
	if err := writer.Issue(recorder, "access", "refresh", issued.Add(accessTTL), refreshTTL); err != nil {
		t.Fatalf("Issue: %v", err)
	}

	cookies := map[string]*http.Cookie{}
	for _, cookie := range recorder.Result().Cookies() {
		cookies[cookie.Name] = cookie
	}

	tests := []struct {
		name         string
		cookie       string
		wantLifetime time.Duration
		wantHTTPOnly bool
		wantPath     string
	}{
		{name: "access", cookie: httphandler.AccessCookie, wantLifetime: accessTTL, wantHTTPOnly: true, wantPath: "/api"},
		{name: "refresh", cookie: httphandler.RefreshCookie, wantLifetime: refreshTTL, wantHTTPOnly: true, wantPath: "/api/v1/auth"},
		{name: "csrf lives as long as refresh", cookie: httphandler.CSRFCookie, wantLifetime: refreshTTL, wantHTTPOnly: false, wantPath: "/"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cookie, ok := cookies[tc.cookie]
			if !ok {
				t.Fatalf("cookie %s was not set", tc.cookie)
			}
			lifetime := cookie.Expires.Sub(issued)
			if lifetime < tc.wantLifetime-tolerance || lifetime > tc.wantLifetime+tolerance {
				t.Errorf("lifetime = %v, want %v", lifetime.Round(time.Second), tc.wantLifetime)
			}
			if cookie.HttpOnly != tc.wantHTTPOnly {
				t.Errorf("HttpOnly = %v, want %v", cookie.HttpOnly, tc.wantHTTPOnly)
			}
			if cookie.Path != tc.wantPath {
				t.Errorf("Path = %q, want %q", cookie.Path, tc.wantPath)
			}
			if !cookie.Secure {
				t.Error("cookie is not Secure")
			}
		})
	}
}

// A browser removes a cookie only when the expiring one matches the name and
// the path it was set with. One that does not match is ignored, and the user
// who signed out still carries a session.
func TestClearExpiresEveryCookieOnItsOwnPath(t *testing.T) {
	recorder := httptest.NewRecorder()
	httphandler.NewCookieWriter(true).Clear(recorder)

	cookies := map[string]*http.Cookie{}
	for _, cookie := range recorder.Result().Cookies() {
		cookies[cookie.Name] = cookie
	}

	tests := []struct {
		name         string
		cookie       string
		wantPath     string
		wantHTTPOnly bool
	}{
		{name: "access", cookie: httphandler.AccessCookie, wantPath: "/api", wantHTTPOnly: true},
		{name: "refresh", cookie: httphandler.RefreshCookie, wantPath: "/api/v1/auth", wantHTTPOnly: true},
		{name: "csrf", cookie: httphandler.CSRFCookie, wantPath: "/", wantHTTPOnly: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cookie, ok := cookies[tc.cookie]
			if !ok {
				t.Fatalf("cookie %s was not cleared", tc.cookie)
			}
			if cookie.MaxAge >= 0 {
				t.Errorf("MaxAge = %d, want negative so the browser drops it", cookie.MaxAge)
			}
			if cookie.Value != "" {
				t.Errorf("Value = %q, want empty", cookie.Value)
			}
			if cookie.Path != tc.wantPath {
				t.Errorf("Path = %q, want %q", cookie.Path, tc.wantPath)
			}
			if cookie.HttpOnly != tc.wantHTTPOnly {
				t.Errorf("HttpOnly = %v, want %v", cookie.HttpOnly, tc.wantHTTPOnly)
			}
		})
	}
}
