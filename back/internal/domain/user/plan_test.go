package user_test

import (
	"testing"

	"github.com/moxicom/cursed_matrix/back/internal/domain/shared"
	"github.com/moxicom/cursed_matrix/back/internal/domain/user"
)

func TestPlanLimits(t *testing.T) {
	tests := []struct {
		name      string
		plan      shared.Plan
		wantTasks int
		wantLinks int
	}{
		{
			name:      "the free plan is capped",
			plan:      shared.PlanFree,
			wantTasks: user.FreeActiveTasks,
			wantLinks: user.FreeTaskLinks,
		},
		{name: "a paid plan is not", plan: shared.PlanPro, wantTasks: user.Unlimited, wantLinks: user.Unlimited},
		{
			name:      "a self-hosted install answers to nobody",
			plan:      shared.PlanSelfHosted,
			wantTasks: user.Unlimited,
			wantLinks: user.Unlimited,
		},
		{
			// Wrong in the direction that costs nobody anything: a plan the
			// code does not recognise must not be handed no limits at all.
			name:      "a plan nobody has heard of is treated as free",
			plan:      shared.Plan("ENTERPRISE"),
			wantTasks: user.FreeActiveTasks,
			wantLinks: user.FreeTaskLinks,
		},
		{name: "and so is an empty one", plan: "", wantTasks: user.FreeActiveTasks, wantLinks: user.FreeTaskLinks},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := user.ActiveTaskLimit(tt.plan); got != tt.wantTasks {
				t.Errorf("ActiveTaskLimit = %d, want %d", got, tt.wantTasks)
			}
			if got := user.TaskLinkLimit(tt.plan); got != tt.wantLinks {
				t.Errorf("TaskLinkLimit = %d, want %d", got, tt.wantLinks)
			}
		})
	}
}
