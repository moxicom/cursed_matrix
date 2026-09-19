package user

import "github.com/moxicom/cursed_matrix/back/internal/domain/shared"

// What the free plan allows. The paid plan lifts both; a self-hosted install
// answers to nobody.
const (
	FreeActiveTasks = 35
	FreeTaskLinks   = 25
)

// Unlimited is the answer for a plan that has no ceiling.
const Unlimited = -1

// ActiveTaskLimit is how many tasks a plan may have open at once.
//
// The cap counts active tasks only, so completing or deleting one frees a
// slot; nothing existing is ever taken away when a plan lapses.
func ActiveTaskLimit(plan shared.Plan) int {
	if plan == shared.PlanFree {
		return FreeActiveTasks
	}
	return Unlimited
}

// TaskLinkLimit is how many links between tasks a plan may hold.
func TaskLinkLimit(plan shared.Plan) int {
	if plan == shared.PlanFree {
		return FreeTaskLinks
	}
	return Unlimited
}
