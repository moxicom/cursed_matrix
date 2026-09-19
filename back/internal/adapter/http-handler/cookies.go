package httphandler

import (
	"crypto/rand"
	"encoding/base64"
	"net/http"
	"time"
)

// Cookie names and paths. The refresh cookie is scoped to the auth path so it
// is sent only by the client's own refresh call, and nothing else in the
// application can leak it by accident.
const (
	AccessCookie  = "cm_access"
	RefreshCookie = "cm_refresh"
	CSRFCookie    = "cm_csrf"
	CSRFHeader    = "X-CSRF-Token"

	apiPath     = "/api"
	refreshPath = "/api/v1/auth"
)

// CookieWriter turns a session into the three cookies of the scheme. Secure is
// configurable only because a developer on plain HTTP would otherwise never
// receive them; in production it is always on.
type CookieWriter struct {
	secure bool
}

// NewCookieWriter builds the writer.
func NewCookieWriter(secure bool) *CookieWriter {
	return &CookieWriter{secure: secure}
}

// Issue sets the access, refresh and CSRF cookies.
func (c *CookieWriter) Issue(w http.ResponseWriter, access, refresh string, accessExpiry time.Time, refreshTTL time.Duration) error {
	csrf, err := randomToken()
	if err != nil {
		return err
	}

	// #nosec G124 -- Secure is a variable only so a developer on plain HTTP
	// receives the cookie at all; outside development it is always true.
	http.SetCookie(w, &http.Cookie{
		Name: AccessCookie, Value: access, Path: apiPath,
		Expires: accessExpiry, HttpOnly: true, Secure: c.secure, SameSite: http.SameSiteLaxMode,
	})
	// #nosec G124 -- as above; this one is additionally SameSite=Strict and
	// scoped to the auth path.
	http.SetCookie(w, &http.Cookie{
		Name: RefreshCookie, Value: refresh, Path: refreshPath,
		Expires: time.Now().Add(refreshTTL), HttpOnly: true, Secure: c.secure, SameSite: http.SameSiteStrictMode,
	})
	// #nosec G124 -- HttpOnly is false by design: the page has to read this
	// value to echo it in a header, which is exactly what a cross-site form
	// cannot do. It carries no authority on its own.
	http.SetCookie(w, &http.Cookie{
		Name: CSRFCookie, Value: csrf, Path: "/",
		Expires: accessExpiry, HttpOnly: false, Secure: c.secure, SameSite: http.SameSiteLaxMode,
	})
	return nil
}

// Clear expires all three cookies.
func (c *CookieWriter) Clear(w http.ResponseWriter) {
	for _, cookie := range []struct{ name, path string }{
		{AccessCookie, apiPath},
		{RefreshCookie, refreshPath},
		{CSRFCookie, "/"},
	} {
		// #nosec G124 -- an expiring cookie carries no value; the flags mirror
		// the ones it was set with so the browser matches and removes it.
		http.SetCookie(w, &http.Cookie{
			Name: cookie.name, Value: "", Path: cookie.path,
			MaxAge: -1, HttpOnly: cookie.name != CSRFCookie, Secure: c.secure,
			SameSite: http.SameSiteLaxMode,
		})
	}
}

func randomToken() (string, error) {
	buffer := make([]byte, 32)
	if _, err := rand.Read(buffer); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buffer), nil
}

// CookieValue reads a cookie, empty when it is not there.
func CookieValue(r *http.Request, name string) string {
	cookie, err := r.Cookie(name)
	if err != nil {
		return ""
	}
	return cookie.Value
}
