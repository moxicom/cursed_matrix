package auth_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/moxicom/cursed_matrix/back/internal/app/auth"
	"github.com/moxicom/cursed_matrix/back/internal/app/cache"
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

type stubRefresh struct{}

func (*stubRefresh) Save(context.Context, uuid.UUID, string, time.Duration) error { return nil }
func (*stubRefresh) Consume(context.Context, uuid.UUID, string) (bool, error)     { return true, nil }
func (*stubRefresh) RevokeAll(context.Context, uuid.UUID) error                   { return nil }

type stubTokens struct{}

func (*stubTokens) Issue(uuid.UUID, shared.Plan) (string, time.Time, error) {
	return "token", time.Now().Add(time.Minute), nil
}

func (*stubTokens) Verify(string) (uuid.UUID, shared.Plan, error) {
	return uuid.Nil, shared.PlanFree, nil
}

func (*stubTokens) VerifyExpired(string) (uuid.UUID, shared.Plan, error) {
	return uuid.Nil, shared.PlanFree, nil
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
				&shared.SystemClock{}, time.Hour)
			if tt.cache != nil {
				configured = tt.cache
				service = auth.NewService(users, &stubRefresh{}, &stubTx{}, &stubTokens{}, configured,
					&shared.SystemClock{}, time.Hour)
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
	hash, err := utils.HashPassword("correct horse battery")
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
				&shared.SystemClock{}, time.Hour)

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
