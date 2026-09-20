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
	if unlimited(plan) {
		return Unlimited
	}
	return FreeActiveTasks
}

// TaskLinkLimit is how many links between tasks a plan may hold.
func TaskLinkLimit(plan shared.Plan) int {
	if unlimited(plan) {
		return Unlimited
	}
	return FreeTaskLinks
}

// unlimited names the plans that lift the caps, rather than naming the one
// that applies them.
//
// Written the other way round, a plan the code does not recognise — a value
// from a newer release, a typo in a migration — would be granted no limits at
// all. This way an unknown plan is treated as the free one, which is wrong in
// the direction that costs nobody anything.
func unlimited(plan shared.Plan) bool {
	return plan == shared.PlanPro || plan == shared.PlanSelfHosted
}
