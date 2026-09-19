package task_test

import (
	"testing"

	"github.com/google/uuid"

	"github.com/moxicom/cursed_matrix/back/internal/domain/shared"
	"github.com/moxicom/cursed_matrix/back/internal/domain/task"
)

func scope(positions ...int32) []task.Task {
	ordered := make([]task.Task, len(positions))
	for i, p := range positions {
		ordered[i] = task.Task{ID: uuid.New(), Position: p}
	}
	return ordered
}

func TestPlace(t *testing.T) {
	tests := []struct {
		name          string
		ordered       []task.Task
		neighbour     func(ordered []task.Task) (before, after *uuid.UUID)
		wantPosition  int32
		wantRebalance []int32
		wantErr       shared.ErrorCode
	}{
		{
			name:         "the first task in an empty scope",
			ordered:      nil,
			wantPosition: task.PositionGap,
		},
		{
			name:         "appending goes past the last",
			ordered:      scope(1024, 2048),
			wantPosition: 3072,
		},
		{
			name:    "before the first lands halfway to zero",
			ordered: scope(1024, 2048),
			neighbour: func(o []task.Task) (*uuid.UUID, *uuid.UUID) {
				return &o[0].ID, nil
			},
			wantPosition: 512,
		},
		{
			name:    "between two neighbours lands in the middle",
			ordered: scope(1024, 2048),
			neighbour: func(o []task.Task) (*uuid.UUID, *uuid.UUID) {
				return &o[1].ID, nil
			},
			wantPosition: 1536,
		},
		{
			name:    "after the last is the same as appending",
			ordered: scope(1024, 2048),
			neighbour: func(o []task.Task) (*uuid.UUID, *uuid.UUID) {
				return nil, &o[1].ID
			},
			wantPosition: 3072,
		},
		{
			name:    "a used-up gap renumbers the scope",
			ordered: scope(1024, 1025),
			neighbour: func(o []task.Task) (*uuid.UUID, *uuid.UUID) {
				return &o[1].ID, nil
			},
			// The placed task takes slot 1; the two existing ones take slots
			// 0 and 2.
			wantPosition:  2048,
			wantRebalance: []int32{1024, 3072},
		},
		{
			name:    "an unknown neighbour is refused",
			ordered: scope(1024),
			neighbour: func([]task.Task) (*uuid.UUID, *uuid.UUID) {
				stranger := uuid.New()
				return &stranger, nil
			},
			wantErr: shared.CodeTaskNotFound,
		},
		{
			name:    "naming both neighbours is ambiguous",
			ordered: scope(1024, 2048),
			neighbour: func(o []task.Task) (*uuid.UUID, *uuid.UUID) {
				return &o[0].ID, &o[1].ID
			},
			wantErr: shared.CodeValidationFailed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var before, after *uuid.UUID
			if tt.neighbour != nil {
				before, after = tt.neighbour(tt.ordered)
			}

			position, placements, err := task.Place(tt.ordered, before, after)

			if tt.wantErr != "" {
				if shared.CodeOf(err) != tt.wantErr {
					t.Fatalf("err = %v, want %s", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("Place: %v", err)
			}
			if position != tt.wantPosition {
				t.Errorf("position = %d, want %d", position, tt.wantPosition)
			}
			if len(placements) != len(tt.wantRebalance) {
				t.Fatalf("%d siblings moved, want %d", len(placements), len(tt.wantRebalance))
			}
			for i, want := range tt.wantRebalance {
				if placements[i].Position != want {
					t.Errorf("sibling %d = %d, want %d", i, placements[i].Position, want)
				}
			}
		})
	}
}

// TestPlaceKeepsTheOrderItWasAskedFor is the property the arithmetic exists
// for: whatever the gaps, the task ends up between the neighbours it named.
func TestPlaceKeepsTheOrderItWasAskedFor(t *testing.T) {
	tests := []struct {
		name    string
		ordered []task.Task
		at      int
	}{
		{name: "roomy gaps", ordered: scope(1024, 2048, 3072), at: 1},
		{name: "no gap at all", ordered: scope(1, 2, 3), at: 1},
		{name: "no gap, at the front", ordered: scope(1, 2, 3), at: 0},
		{name: "no gap, at the back", ordered: scope(1, 2, 3), at: 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			before := tt.ordered[tt.at].ID
			position, placements, err := task.Place(tt.ordered, &before, nil)
			if err != nil {
				t.Fatalf("Place: %v", err)
			}

			final := make(map[uuid.UUID]int32, len(tt.ordered))
			for i := range tt.ordered {
				final[tt.ordered[i].ID] = tt.ordered[i].Position
			}
			for _, placement := range placements {
				final[placement.ID] = placement.Position
			}

			if tt.at > 0 {
				if left := final[tt.ordered[tt.at-1].ID]; left >= position {
					t.Errorf("the task landed at %d, not after its left neighbour at %d", position, left)
				}
			}
			if right := final[before]; right <= position {
				t.Errorf("the task landed at %d, not before %d", position, right)
			}
		})
	}
}
