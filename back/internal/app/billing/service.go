// Package billing decides what a plan costs and what buying one does.
package billing

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/moxicom/cursed_matrix/back/internal/app/cache"
	"github.com/moxicom/cursed_matrix/back/internal/app/port"
	"github.com/moxicom/cursed_matrix/back/internal/domain/shared"
	"github.com/moxicom/cursed_matrix/back/internal/domain/user"
)

type Price struct {
	Language shared.Language
	Currency string
	Amount   int64
}

type Offer struct {
	Plan            shared.Plan
	Prices          []Price
	ActiveTaskLimit int
	TaskLinkLimit   int
}

// Config is what the operator decided.
type Config struct {
	Enabled bool

	GrantedPeriod time.Duration
	Prices        []Price
}

type Service struct {
	users  port.UserRepository
	cache  port.Cache
	config Config
	clock  shared.Clock
}

func NewService(users port.UserRepository, cached port.Cache, config Config, clock shared.Clock) *Service {
	return &Service{users: users, cache: cached, config: config, clock: clock}
}

func (s *Service) Enabled() bool { return s.config.Enabled }

func (s *Service) Offers() []Offer {
	return []Offer{
		{
			Plan:            shared.PlanFree,
			ActiveTaskLimit: user.ActiveTaskLimit(shared.PlanFree),
			TaskLinkLimit:   user.TaskLinkLimit(shared.PlanFree),
		},
		{
			Plan:            shared.PlanPro,
			Prices:          s.config.Prices,
			ActiveTaskLimit: user.ActiveTaskLimit(shared.PlanPro),
			TaskLinkLimit:   user.TaskLinkLimit(shared.PlanPro),
		},
	}
}

// Checkout buys a plan.
//
// While billing is off this grants it outright, which is the whole point of
// the switch: everything downstream of a purchase — the gate lifting, the
// quotas going away, the plan showing on the account — can be built and used
// before anyone is charged. With billing on and no provider wired, the
// purchase is refused rather than accepted: taking an instruction to charge
// that nothing will act on is worse than saying no.
//
// A granted plan does not stack. Extending rather than replacing is right for
// a payment — time already paid for must not be burnt — but nothing was paid
// here, so a client in a loop would otherwise turn one month into as many
// years as it cares to ask for, quotas and all.
func (s *Service) Checkout(ctx context.Context, userID uuid.UUID) (user.Subscription, error) {
	if s.config.Enabled {
		return user.Subscription{}, shared.NewError(shared.CodeBillingUnavailable, nil)
	}

	if _, err := s.users.ByID(ctx, userID); err != nil {
		return user.Subscription{}, err
	}

	granted := user.Granted(shared.PlanPro, s.config.GrantedPeriod, s.clock.Now().UTC())
	if err := s.users.SetSubscription(ctx, userID, granted); err != nil {
		return user.Subscription{}, err
	}

	s.forget(ctx, userID)
	return granted, nil
}

func (s *Service) forget(ctx context.Context, userID uuid.UUID) {
	if s.cache == nil {
		return
	}
	_ = s.cache.Invalidate(ctx, cache.UserScope(userID))
}
