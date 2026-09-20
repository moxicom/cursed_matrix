// Package token signs and checks access tokens. The algorithm is an
// implementation detail: the application depends on port.TokenIssuer.
package token

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"github.com/moxicom/cursed_matrix/back/internal/domain/shared"
	"github.com/moxicom/cursed_matrix/back/internal/domain/user"

	"github.com/moxicom/cursed_matrix/back/internal/app/port"
)

// Claims is what an access token asserts. Nothing here is a secret: a JWT is
// signed, not encrypted, and the page could read it if it were not in an
// HttpOnly cookie.
type Claims struct {
	jwt.RegisteredClaims

	Plan      shared.Plan `json:"plan"`
	PlanUntil *int64      `json:"planUntil,omitempty"`
}

// TokenIssuer signs and verifies access tokens.
type JWTIssuer struct {
	secret []byte
	ttl    time.Duration
	clock  shared.Clock
}

// NewJWTIssuer builds an issuer over the signing secret.
func NewJWTIssuer(secret string, ttl time.Duration, clock shared.Clock) *JWTIssuer {
	return &JWTIssuer{secret: []byte(secret), ttl: ttl, clock: clock}
}

// Issue signs an access token for the account.
func (t *JWTIssuer) Issue(userID uuid.UUID, subscription user.Subscription) (string, time.Time, error) {
	now := t.clock.Now()
	expiry := now.Add(t.ttl)

	claims := Claims{
		Subject:   userID.String(),
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(expiry),
		ID:        uuid.NewString(),
		Plan:      subscription.Plan,
	}
	if subscription.ExpiresAt != nil {
		until := subscription.ExpiresAt.Unix()
		claims.PlanUntil = &until
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	signed, err := token.SignedString(t.secret)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("sign token: %w", err)
	}
	return signed, expiry, nil
}

// Verify checks the signature and the expiry, and returns what the token
// asserts. Every failure answers the same code: a client cannot be told whether
// its token was forged, expired or truncated.
func (t *JWTIssuer) Verify(raw string) (uuid.UUID, user.Subscription, error) {
	claims := &Claims{}

	_, err := jwt.ParseWithClaims(raw, claims, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return t.secret, nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}))
	if err != nil {
		return uuid.Nil, user.Subscription{}, shared.WrapError(err, shared.CodeSessionExpired, nil)
	}
	return subscriptionOf(claims)
}

func subscriptionOf(claims *Claims) (uuid.UUID, user.Subscription, error) {
	userID, err := uuid.Parse(claims.Subject)
	if err != nil {
		return uuid.Nil, user.Subscription{}, shared.NewError(shared.CodeSessionExpired, nil)
	}

	subscription := user.Subscription{Plan: claims.Plan}
	if claims.PlanUntil != nil {
		until := time.Unix(*claims.PlanUntil, 0).UTC()
		subscription.ExpiresAt = &until
	}
	return userID, subscription, nil
}

// VerifyExpired checks the signature but tolerates expiry, which is what the
// refresh endpoint needs: the token it is handed is expected to be stale, and
// the refresh cookie is the credential that matters there.
func (t *JWTIssuer) VerifyExpired(raw string) (uuid.UUID, user.Subscription, error) {
	claims := &Claims{}

	_, err := jwt.ParseWithClaims(raw, claims, func(token *jwt.Token) (any, error) {
		return t.secret, nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}), jwt.WithoutClaimsValidation())
	if err != nil {
		return uuid.Nil, user.Subscription{}, shared.WrapError(err, shared.CodeSessionExpired, nil)
	}
	return subscriptionOf(claims)
}

// TTL is how long an issued token stays valid, which is also the window in
// which a revocation has not taken effect yet.
func (t *JWTIssuer) TTL() time.Duration { return t.ttl }

var _ port.TokenIssuer = (*JWTIssuer)(nil)
