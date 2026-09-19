package progression_test

import (
	"testing"

	"github.com/moxicom/cursed_matrix/back/internal/domain/progression"
	"github.com/moxicom/cursed_matrix/back/internal/domain/shared"
)

func TestReward(t *testing.T) {
	tests := []struct {
		name      string
		quadrant  shared.Quadrant
		isSubtask bool
		want      int32
		wantErr   bool
	}{
		{name: "Q1 task", quadrant: shared.QuadrantImportantUrgent, want: 50},
		{name: "Q2 task", quadrant: shared.QuadrantImportantNotUrgent, want: 35},
		{name: "Q3 task", quadrant: shared.QuadrantNotImportantUrgent, want: 20},
		{name: "Q4 task", quadrant: shared.QuadrantNotImportantNotUrgnt, want: 10},
		{name: "Q1 subtask", quadrant: shared.QuadrantImportantUrgent, isSubtask: true, want: 18},
		{name: "Q2 subtask", quadrant: shared.QuadrantImportantNotUrgent, isSubtask: true, want: 12},
		{name: "Q3 subtask", quadrant: shared.QuadrantNotImportantUrgent, isSubtask: true, want: 7},
		{name: "Q4 subtask", quadrant: shared.QuadrantNotImportantNotUrgnt, isSubtask: true, want: 4},
		{name: "unknown quadrant grants nothing", quadrant: "SOMEDAY_MAYBE", wantErr: true},
		{name: "empty quadrant grants nothing", quadrant: "", wantErr: true},
	}

	cfg := progression.DefaultConfig()
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := cfg.Reward(tc.quadrant, tc.isSubtask)
			if (err != nil) != tc.wantErr {
				t.Fatalf("Reward(%q, %v) error = %v, wantErr %v", tc.quadrant, tc.isSubtask, err, tc.wantErr)
			}
			if !tc.wantErr && got != tc.want {
				t.Errorf("Reward(%q, %v) = %d, want %d", tc.quadrant, tc.isSubtask, got, tc.want)
			}
		})
	}
}

func TestRewardOrdering(t *testing.T) {
	tests := []struct {
		name            string
		higher, lower   shared.Quadrant
		higherIsSubtask bool
		lowerIsSubtask  bool
	}{
		{name: "Q1 over Q2", higher: shared.QuadrantImportantUrgent, lower: shared.QuadrantImportantNotUrgent},
		{name: "Q2 over Q3", higher: shared.QuadrantImportantNotUrgent, lower: shared.QuadrantNotImportantUrgent},
		{name: "Q3 over Q4", higher: shared.QuadrantNotImportantUrgent, lower: shared.QuadrantNotImportantNotUrgnt},
		{
			name: "task over subtask in Q1", higher: shared.QuadrantImportantUrgent,
			lower: shared.QuadrantImportantUrgent, lowerIsSubtask: true,
		},
		{
			name: "task over subtask in Q4", higher: shared.QuadrantNotImportantNotUrgnt,
			lower: shared.QuadrantNotImportantNotUrgnt, lowerIsSubtask: true,
		},
	}

	cfg := progression.DefaultConfig()
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			higher, err := cfg.Reward(tc.higher, tc.higherIsSubtask)
			if err != nil {
				t.Fatalf("Reward: %v", err)
			}
			lower, err := cfg.Reward(tc.lower, tc.lowerIsSubtask)
			if err != nil {
				t.Fatalf("Reward: %v", err)
			}
			if higher <= lower {
				t.Errorf("%s awards %d, %s awards %d: the first must award more",
					tc.higher, higher, tc.lower, lower)
			}
		})
	}
}

func TestXPForLevel(t *testing.T) {
	tests := []struct {
		name  string
		level int32
		want  int64
	}{
		{name: "level 1 starts at zero", level: 1, want: 0},
		{name: "level 0 is clamped", level: 0, want: 0},
		{name: "negative level is clamped", level: -5, want: 0},
		{name: "level 2", level: 2, want: 45},
		{name: "level 3", level: 3, want: 180},
		{name: "level 5", level: 5, want: 720},
		{name: "level 10", level: 10, want: 3645},
		{name: "level 20", level: 20, want: 16245},
	}

	cfg := progression.DefaultConfig()
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := cfg.XPForLevel(tc.level); got != tc.want {
				t.Errorf("XPForLevel(%d) = %d, want %d", tc.level, got, tc.want)
			}
		})
	}
}

func TestLevelForXP(t *testing.T) {
	tests := []struct {
		name string
		xp   int64
		want int32
	}{
		{name: "zero is level 1", xp: 0, want: 1},
		{name: "negative is level 1", xp: -10, want: 1},
		{name: "just below the second threshold", xp: 44, want: 1},
		{name: "exactly the second threshold", xp: 45, want: 2},
		{name: "just below the third", xp: 179, want: 2},
		{name: "exactly the third", xp: 180, want: 3},
		{name: "the level a stored 12 disagrees with", xp: 7420, want: 13},
		{name: "level 20 threshold", xp: 16245, want: 20},
	}

	cfg := progression.DefaultConfig()
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := cfg.LevelForXP(tc.xp); got != tc.want {
				t.Errorf("LevelForXP(%d) = %d, want %d", tc.xp, got, tc.want)
			}
		})
	}
}

func TestLevelCurveIsSelfConsistent(t *testing.T) {
	cfg := progression.DefaultConfig()

	for level := int32(1); level <= 40; level++ {
		at := cfg.XPForLevel(level)
		if got := cfg.LevelForXP(at); got != level {
			t.Errorf("LevelForXP(XPForLevel(%d)=%d) = %d, want %d", level, at, got, level)
		}
		if level > 1 {
			if got := cfg.LevelForXP(at - 1); got != level-1 {
				t.Errorf("LevelForXP(%d) = %d, want %d one below the boundary", at-1, got, level-1)
			}
		}
	}
}

func TestProgress(t *testing.T) {
	tests := []struct {
		name string
		xp   int64
		want progression.Progress
	}{
		{name: "start of level 1", xp: 0, want: progression.Progress{Level: 1, Into: 0, Span: 45, Remaining: 45}},
		{name: "inside level 3", xp: 200, want: progression.Progress{Level: 3, Into: 20, Span: 225, Remaining: 205}},
		{name: "exactly on a threshold", xp: 180, want: progression.Progress{Level: 3, Into: 0, Span: 225, Remaining: 225}},
		{name: "one below a threshold", xp: 179, want: progression.Progress{Level: 2, Into: 134, Span: 135, Remaining: 1}},
	}

	cfg := progression.DefaultConfig()
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := cfg.Progress(tc.xp); got != tc.want {
				t.Errorf("Progress(%d) = %+v, want %+v", tc.xp, got, tc.want)
			}
		})
	}
}
