package auth_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/moxicom/cursed_matrix/back/internal/app/auth"
	"github.com/moxicom/cursed_matrix/back/internal/app/cache"
	"github.com/moxicom/cursed_matrix/back/internal/app/session"
	"github.com/moxicom/cursed_matrix/back/internal/domain/shared"
	"github.com/moxicom/cursed_matrix/back/internal/domain/user"
	"github.com/moxicom/cursed_matrix/back/pkg/utils"
)

type stubUsers struct {
	account  *user.User
	written  *user.Settings
	writeErr error
	touched  bool
}

func (s *stubUsers) ByID(context.Context, uuid.UUID) (*user.User, error) {
	if s.account == nil {
		return nil, shared.NewError(shared.CodeUserNotFound, nil)
	}
	copied := *s.account
	return &copied, nil
}

func (s *stubUsers) ByUsername(context.Context, string) (*user.User, error) {
	return s.ByID(context.Background(), uuid.Nil)
}
func (s *stubUsers) Create(context.Context, *user.User) error { return nil }

func (s *stubUsers) UpdateSettings(_ context.Context, _ uuid.UUID, settings user.Settings) error {
	if s.writeErr != nil {
		return s.writeErr
	}
	s.written = &settings
	s.account.Settings = settings
	return nil
}

func (s *stubUsers) TouchLogin(context.Context, uuid.UUID, time.Time) error {
	s.touched = true
	return nil
}

func (*stubUsers) ApplyStats(context.Context, uuid.UUID, user.StatsDelta) (user.Stats, error) {
	return user.Stats{}, nil
}
func (*stubUsers) SetLevel(context.Context, uuid.UUID, int32) error       { return nil }
func (*stubUsers) LockAccount(context.Context, uuid.UUID) error           { return nil }
func (*stubUsers) SoftDelete(context.Context, uuid.UUID, time.Time) error { return nil }

func (*stubUsers) SetSubscription(context.Context, uuid.UUID, user.Subscription) error {
	return nil
}

func (*stubUsers) TouchStreak(context.Context, uuid.UUID, time.Time) (user.StreakChange, error) {
	return user.StreakChange{}, nil
}

type stubRefresh struct {
	blocked bool
	// outcome is what Consume answers; the zero value is an unknown token, so
	// the tests that refresh set it and the rest never reach it.
	outcome session.RefreshOutcome
	// consumedWith is the grace Consume was asked to honour.
	consumedWith time.Duration
	revoked      bool
	dropped      bool
	saved        int
}

func (s *stubRefresh) Save(context.Context, uuid.UUID, string, time.Duration) error {
	s.saved++
	return nil
}

func (s *stubRefresh) Consume(_ context.Context, _ uuid.UUID, _ string, grace time.Duration) (session.RefreshOutcome, error) {
	s.consumedWith = grace
	return s.outcome, nil
}

func (s *stubRefresh) Drop(context.Context, uuid.UUID, string) error {
	s.dropped = true
	return nil
}

func (s *stubRefresh) RevokeAll(context.Context, uuid.UUID) error {
	s.revoked = true
	return nil
}

func (s *stubRefresh) BlockAccess(context.Context, uuid.UUID, time.Duration) error {
	s.blocked = true
	return nil
}

func (s *stubRefresh) AccessBlocked(context.Context, uuid.UUID) (bool, error) {
	return s.blocked, nil
}

type stubTokens struct{}

func (*stubTokens) Issue(uuid.UUID, user.Subscription) (string, time.Time, error) {
	return "token", time.Now().Add(time.Minute), nil
}

func (*stubTokens) Verify(string) (uuid.UUID, user.Subscription, error) {
	return uuid.Nil, user.Subscription{Plan: shared.PlanFree}, nil
}

func (*stubTokens) VerifyExpired(string) (uuid.UUID, user.Subscription, error) {
	return uuid.Nil, user.Subscription{Plan: shared.PlanFree}, nil
}
func (*stubTokens) TTL() time.Duration { return time.Minute }

type stubTx struct{}

func (*stubTx) Do(ctx context.Context, fn func(context.Context) error) error { return fn(ctx) }

type stubCache struct {
	invalidated []cache.Scope
	err         error
}

func (c *stubCache) GetInto(context.Context, cache.Key, any) (bool, error) { return false, nil }

func (c *stubCache) Set(context.Context, cache.Key, any, time.Duration) error { return nil }

func (c *stubCache) Invalidate(_ context.Context, scope cache.Scope) error {
	if c.err != nil {
		return c.err
	}
	c.invalidated = append(c.invalidated, scope)
	return nil
}

// TestUpdateSettingsRetiresTheCache covers the two halves of the same rule: a
// preference change must never be served from a stale entry, and a cache that
// is down must never turn a committed write into a failure the client sees.
func TestUpdateSettingsRetiresTheCache(t *testing.T) {
	userID := uuid.New()
	moscow := "Europe/Moscow"

	tests := []struct {
		name        string
		cache       *stubCache
		writeErr    error
		wantErr     bool
		wantRetired int
	}{
		{
			name:        "a successful change retires the scope",
			cache:       &stubCache{},
			wantRetired: 1,
		},
		{
			name:        "a cache that is down does not fail the write",
			cache:       &stubCache{err: errors.New("redis is down")},
			wantRetired: 0,
		},
		{
			name:  "no cache configured is not an error",
			cache: nil,
		},
		{
			name:     "a failed write is still a failure",
			cache:    &stubCache{},
			writeErr: shared.NewError(shared.CodeInternal, nil),
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			users := &stubUsers{
				account:  &user.User{ID: userID, Settings: user.Settings{Timezone: "UTC"}},
				writeErr: tt.writeErr,
			}

			var configured *stubCache
			service := auth.NewService(users, &stubRefresh{}, &stubTx{}, &stubTokens{}, nil,
				&shared.SystemClock{}, time.Hour, 0, 14*24*time.Hour)
			if tt.cache != nil {
				configured = tt.cache
				service = auth.NewService(users, &stubRefresh{}, &stubTx{}, &stubTokens{}, configured,
					&shared.SystemClock{}, time.Hour, 0, 14*24*time.Hour)
			}

			_, err := service.UpdateSettings(context.Background(), userID,
				auth.SettingsChange{Timezone: &moscow})

			if tt.wantErr {
				if err == nil {
					t.Fatal("the write failed and the caller was told it succeeded")
				}
				return
			}
			if err != nil {
				t.Fatalf("UpdateSettings: %v", err)
			}
			if users.written == nil || users.written.Timezone != moscow {
				t.Errorf("stored timezone = %v, want %q", users.written, moscow)
			}
			if configured != nil && len(configured.invalidated) != tt.wantRetired {
				t.Errorf("retired %d scopes, want %d", len(configured.invalidated), tt.wantRetired)
			}
			if tt.wantRetired > 0 && configured.invalidated[0] != cache.UserScope(userID) {
				t.Errorf("retired %q, want the account's own scope", configured.invalidated[0])
			}
		})
	}
}

// TestLoginRetiresTheCacheWhenTheZoneChanges covers the one write that changes
// a preference outside UpdateSettings: a traveller signing in from a new
// timezone. Missing it there would leave the board reading deadlines in the
// old zone until the entry expired on its own.
func TestLoginRetiresTheCacheWhenTheZoneChanges(t *testing.T) {
	hash, err := utils.HashPassword(t.Context(), "correct horse battery")
	if err != nil {
		t.Fatalf("hash: %v", err)
	}

	tests := []struct {
		name        string
		stored      string
		sent        string
		wantStored  string
		wantRetired int
	}{
		{
			name:        "a new zone is written and the scope retired",
			stored:      "UTC",
			sent:        "Asia/Tokyo",
			wantStored:  "Asia/Tokyo",
			wantRetired: 1,
		},
		{
			name:       "the same zone touches nothing",
			stored:     "Asia/Tokyo",
			sent:       "Asia/Tokyo",
			wantStored: "Asia/Tokyo",
		},
		{
			name:       "a client that sends no zone leaves the stored one alone",
			stored:     "Asia/Tokyo",
			sent:       "",
			wantStored: "Asia/Tokyo",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			users := &stubUsers{account: &user.User{
				ID:           uuid.New(),
				Username:     "traveller",
				PasswordHash: hash,
				Settings:     user.Settings{Timezone: tt.stored},
			}}
			cached := &stubCache{}
			service := auth.NewService(users, &stubRefresh{}, &stubTx{}, &stubTokens{}, cached,
				&shared.SystemClock{}, time.Hour, 0, 14*24*time.Hour)

			if _, err := service.Login(context.Background(), "traveller", "correct horse battery", tt.sent); err != nil {
				t.Fatalf("Login: %v", err)
			}

			if users.account.Settings.Timezone != tt.wantStored {
				t.Errorf("stored timezone = %q, want %q", users.account.Settings.Timezone, tt.wantStored)
			}
			if len(cached.invalidated) != tt.wantRetired {
				t.Errorf("retired %d scopes, want %d", len(cached.invalidated), tt.wantRetired)
			}
			if !users.touched {
				t.Error("the login was not recorded")
			}
		})
	}
}

// A refresh token presented twice is a replay and costs the account every
// session — except right after the rotation, where the second presenter is
// almost always the first one again and is answered with a fresh pair.
func TestRefreshJudgesASpentToken(t *testing.T) {
	const grace = 30 * time.Second
	userID := uuid.New()

	tests := []struct {
		name        string
		outcome     session.RefreshOutcome
		wantSession bool
		wantRevoked bool
	}{
		{name: "a live token is rotated", outcome: session.RefreshValid, wantSession: true},
		{name: "a token spent a moment ago is answered again", outcome: session.RefreshJustRotated, wantSession: true},
		{name: "an unknown token revokes the family", outcome: session.RefreshUnknown, wantRevoked: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			users := &stubUsers{account: &user.User{ID: userID, Settings: user.Settings{Timezone: "UTC"}}}
			refresh := &stubRefresh{outcome: tt.outcome}
			service := auth.NewService(users, refresh, &stubTx{}, &stubTokens{}, &stubCache{},
				&shared.SystemClock{}, time.Hour, grace, 14*24*time.Hour)

			session, err := service.Refresh(context.Background(), userID, "presented")

			if tt.wantSession {
				if err != nil || session == nil {
					t.Fatalf("Refresh = %v, %v; want a session", session, err)
				}
				if refresh.saved != 1 {
					t.Errorf("%d refresh tokens saved, want 1", refresh.saved)
				}
			} else if shared.CodeOf(err) != shared.CodeSessionExpired {
				t.Fatalf("Refresh error = %v, want %s", err, shared.CodeSessionExpired)
			}
			if refresh.revoked != tt.wantRevoked {
				t.Errorf("revoked = %v, want %v", refresh.revoked, tt.wantRevoked)
			}
			if refresh.consumedWith != grace {
				t.Errorf("Consume was given a grace of %v, want %v", refresh.consumedWith, grace)
			}
		})
	}
}

// Signing out must not leave the token honoured for the grace window: Consume
// would, Drop does not.
func TestLogoutDropsTheTokenRatherThanConsumingIt(t *testing.T) {
	tests := []struct {
		name  string
		grace time.Duration
	}{
		{name: "with a grace window", grace: 30 * time.Second},
		{name: "without one", grace: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			refresh := &stubRefresh{outcome: session.RefreshValid}
			service := auth.NewService(&stubUsers{}, refresh, &stubTx{}, &stubTokens{}, &stubCache{},
				&shared.SystemClock{}, time.Hour, tt.grace, 14*24*time.Hour)

			if err := service.Logout(context.Background(), uuid.New(), "presented"); err != nil {
				t.Fatalf("Logout: %v", err)
			}
			if !refresh.dropped {
				t.Error("the token was not dropped")
			}
			if refresh.consumedWith != 0 || refresh.saved != 0 {
				t.Error("Logout went through Consume or issued a token")
			}
		})
	}
}
