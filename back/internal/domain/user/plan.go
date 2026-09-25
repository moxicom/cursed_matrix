package user

import "github.com/moxicom/cursed_matrix/back/internal/domain/shared"

const (
	FreeActiveTasks = 10
	FreeTaskLinks   = 10
)

const Unlimited = -1

func ActiveTaskLimit(plan shared.Plan) int {
	if unlimited(plan) {
		return Unlimited
	}
	return FreeActiveTasks
}

func TaskLinkLimit(plan shared.Plan) int {
	if unlimited(plan) {
		return Unlimited
	}
	return FreeTaskLinks
}

func unlimited(plan shared.Plan) bool {
	return plan == shared.PlanPro || plan == shared.PlanSelfHosted
}
