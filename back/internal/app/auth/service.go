package auth

import (
	"context"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/moxicom/cursed_matrix/back/internal/app/port"
	"github.com/moxicom/cursed_matrix/back/internal/domain/shared"
	"github.com/moxicom/cursed_matrix/back/internal/domain/user"
	"github.com/moxicom/cursed_matrix/back/pkg/utils"
)

// Input limits. The username is the credential and appears in the public
// leaderboard, so it is bounded on both ends; the password only has a floor,
// because a long passphrase is a good password.
const (
	UsernameMinLen = 3
	UsernameMaxLen = 32
	PasswordMinLen = 12
	PasswordMaxLen = 128
)

// Session is what a successful sign-in hands back: the account, and the two
// tokens the transport turns into cookies.
type Session struct {
	User         *user.User
	AccessToken  string
	AccessExpiry time.Time
	RefreshToken string
}

// Credentials is the signing-in half of registration.
type Credentials struct {
	Username string
	Password string
	Timezone string
	Language shared.Language
	Email    *string
}

// Service carries out the account use cases.
type Service struct {
	users      port.UserRepository
	refresh    port.RefreshStore
	tx         port.TxManager
	tokens     port.TokenIssuer
	clock      shared.Clock
	refreshTTL time.Duration
}

// NewService wires the use cases to their ports.
func NewService(
	users port.UserRepository,
	refresh port.RefreshStore,
	tx port.TxManager,
	tokens port.TokenIssuer,
	clock shared.Clock,
	refreshTTL time.Duration,
) *Service {
	return &Service{users: users, refresh: refresh, tx: tx, tokens: tokens, clock: clock, refreshTTL: refreshTTL}
}

// Register creates an account and signs it in.
func (s *Service) Register(ctx context.Context, creds Credentials) (*Session, error) {
	username, err := normalizeUsername(creds.Username)
	if err != nil {
		return nil, err
	}
	if err := checkPassword(creds.Password); err != nil {
		return nil, err
	}

	hash, err := utils.HashPassword(creds.Password)
	if err != nil {
		return nil, shared.WrapError(err, shared.CodeInternal, nil)
	}

	language := creds.Language
	if language == "" {
		language = shared.LanguageEN
	}

	account := &user.User{
		ID:           uuid.New(),
		Username:     username,
		Email:        normalizeEmail(creds.Email),
		PasswordHash: hash,
		CreatedAt:    s.clock.Now(),
		Settings: user.Settings{
			Language:             language,
			Timezone:             normalizeTimezone(creds.Timezone),
			ShowInLeaderboard:    false,
			NotificationsEnabled: true,
		},
	}

	if err := s.tx.Do(ctx, func(ctx context.Context) error {
		return s.users.Create(ctx, account)
	}); err != nil {
		return nil, err
	}

	// Read the account back rather than returning what was written: the
	// aggregate row carries defaults the service does not set, and level starts
	// at one, not at zero.
	stored, err := s.users.ByID(ctx, account.ID)
	if err != nil {
		return nil, err
	}
	return s.issue(ctx, stored)
}

// Login verifies the credentials. An unknown username costs the same time as a
// wrong password, so the endpoint cannot be used to discover which usernames
// are taken.
func (s *Service) Login(ctx context.Context, username, password, timezone string) (*Session, error) {
	account, err := s.users.ByUsername(ctx, strings.TrimSpace(username))
	if err != nil {
		if shared.CodeOf(err) == shared.CodeUserNotFound {
			_ = utils.VerifyAbsentAccount(password)
			return nil, shared.NewError(shared.CodeInvalidCredentials, nil)
		}
		return nil, err
	}

	if err := utils.VerifyPassword(password, account.PasswordHash); err != nil {
		if errors.Is(err, utils.ErrPasswordMismatch) {
			return nil, shared.NewError(shared.CodeInvalidCredentials, nil)
		}
		return nil, shared.WrapError(err, shared.CodeInternal, nil)
	}

	now := s.clock.Now()
	if err := s.users.TouchLogin(ctx, account.ID, now); err != nil {
		return nil, err
	}

	if zone := normalizeTimezone(timezone); zone != account.Settings.Timezone && timezone != "" {
		account.Settings.Timezone = zone
		if err := s.users.UpdateSettings(ctx, account.ID, account.Settings); err != nil {
			return nil, err
		}
	}

	return s.issue(ctx, account)
}

// AccountFromRefreshToken reads the account out of a refresh token. The value
// is still opaque to the client: knowing the id grants nothing without the
// random half, which only the store has seen.
func AccountFromRefreshToken(token string) (uuid.UUID, error) {
	id, _, found := strings.Cut(token, ".")
	if !found {
		return uuid.Nil, shared.NewError(shared.CodeSessionExpired, nil)
	}
	parsed, err := uuid.Parse(id)
	if err != nil {
		return uuid.Nil, shared.NewError(shared.CodeSessionExpired, nil)
	}
	return parsed, nil
}

// Refresh rotates the pair. A token presented twice is a replay: the whole
// family is revoked, because the second presenter is either the attacker or the
// victim and there is no way to tell which.
func (s *Service) Refresh(ctx context.Context, userID uuid.UUID, refreshToken string) (*Session, error) {
	valid, err := s.refresh.Consume(ctx, userID, refreshToken)
	if err != nil {
		return nil, err
	}
	if !valid {
		if err := s.refresh.RevokeAll(ctx, userID); err != nil {
			return nil, err
		}
		return nil, shared.NewError(shared.CodeSessionExpired, nil)
	}

	account, err := s.users.ByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	return s.issue(ctx, account)
}

// Logout consumes the presented token. It answers the same way whether or not
// there was a session to end.
func (s *Service) Logout(ctx context.Context, userID uuid.UUID, refreshToken string) error {
	_, err := s.refresh.Consume(ctx, userID, refreshToken)
	return err
}

// LogoutAll revokes every refresh token of the account.
func (s *Service) LogoutAll(ctx context.Context, userID uuid.UUID) error {
	return s.refresh.RevokeAll(ctx, userID)
}

// Account is the payload the client boots from.
func (s *Service) Account(ctx context.Context, userID uuid.UUID) (*user.User, error) {
	return s.users.ByID(ctx, userID)
}

// UpdateSettings changes the preferences that belong to the account.
func (s *Service) UpdateSettings(ctx context.Context, userID uuid.UUID, change SettingsChange) (*user.User, error) {
	account, err := s.users.ByID(ctx, userID)
	if err != nil {
		return nil, err
	}

	if change.Language != nil {
		if !change.Language.Valid() {
			return nil, shared.NewError(shared.CodeValidationFailed,
				map[string]any{"field": "language", "value": string(*change.Language)})
		}
		account.Settings.Language = *change.Language
	}
	if change.Timezone != nil {
		account.Settings.Timezone = normalizeTimezone(*change.Timezone)
	}
	if change.ShowInLeaderboard != nil {
		account.Settings.ShowInLeaderboard = *change.ShowInLeaderboard
	}

	if err := s.users.UpdateSettings(ctx, userID, account.Settings); err != nil {
		return nil, err
	}
	return s.users.ByID(ctx, userID)
}

// SettingsChange carries only the preferences the client sent.
type SettingsChange struct {
	Language          *shared.Language
	Timezone          *string
	ShowInLeaderboard *bool
}

func (s *Service) issue(ctx context.Context, account *user.User) (*Session, error) {
	access, expiry, err := s.tokens.Issue(account.ID, shared.PlanFree)
	if err != nil {
		return nil, shared.WrapError(err, shared.CodeInternal, nil)
	}

	// The token carries the account it belongs to, so refreshing and logging out
	// work from the refresh cookie alone. Identifying the caller by the access
	// cookie would break both the moment that cookie expires — which is fifteen
	// minutes, while the refresh cookie lives thirty days.
	refreshToken := account.ID.String() + "." + uuid.NewString()
	if err := s.refresh.Save(ctx, account.ID, refreshToken, s.refreshTTL); err != nil {
		return nil, err
	}

	return &Session{
		User:         account,
		AccessToken:  access,
		AccessExpiry: expiry,
		RefreshToken: refreshToken,
	}, nil
}

func normalizeUsername(raw string) (string, error) {
	username := strings.TrimSpace(raw)
	length := utf8.RuneCountInString(username)

	if length < UsernameMinLen || length > UsernameMaxLen {
		return "", shared.NewError(shared.CodeValidationFailed,
			map[string]any{"field": "username", "min": UsernameMinLen, "max": UsernameMaxLen})
	}
	if strings.ContainsAny(username, " \t\n\r") {
		return "", shared.NewError(shared.CodeValidationFailed,
			map[string]any{"field": "username", "reason": "whitespace"})
	}
	return username, nil
}

func checkPassword(password string) error {
	length := utf8.RuneCountInString(password)
	if length < PasswordMinLen || length > PasswordMaxLen {
		return shared.NewError(shared.CodeValidationFailed,
			map[string]any{"field": "password", "min": PasswordMinLen, "max": PasswordMaxLen})
	}
	return nil
}

// normalizeTimezone keeps an unknown zone from failing the request: losing a
// streak to a typo in a header is worse than computing one day in UTC.
func normalizeTimezone(zone string) string {
	if zone == "" {
		return "UTC"
	}
	if _, err := time.LoadLocation(zone); err != nil {
		return "UTC"
	}
	return zone
}

func normalizeEmail(email *string) *string {
	if email == nil {
		return nil
	}
	trimmed := strings.ToLower(strings.TrimSpace(*email))
	if trimmed == "" {
		return nil
	}
	return &trimmed
}
