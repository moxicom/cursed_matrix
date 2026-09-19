package task

import (
	"github.com/google/uuid"

	"github.com/moxicom/cursed_matrix/back/internal/domain/shared"
)

// PositionCeiling is where appending stops climbing and the scope is
// renumbered instead.
//
// Every append sits one gap past the last, so a list that is added to for
// years walks upwards even as tasks are completed — completed tasks keep their
// place. Left alone it would eventually pass what the column can hold and wrap
// silently, which reorders the board with no error anywhere. Renumbering costs
// one extra write and bounds the value by the size of the list.
const PositionCeiling = 1 << 30

// Placement is a task that has to be written back at a new position.
type Placement struct {
	ID       uuid.UUID
	Position int32
}

// Place decides where a task goes in an ordered list.
//
// The client names a neighbour, never a number: two clients dragging at once
// would compute the same number from different starting points, and the second
// write would silently land in the wrong place. Positions are spaced by
// PositionGap so an insertion is usually one write; when a gap is used up the
// scope is renumbered and every moved sibling comes back in rebalanced.
//
// ordered must be the target scope without the task being placed, sorted by
// position.
func Place(ordered []Task, before, after *uuid.UUID) (int32, []Placement, error) {
	if before != nil && after != nil {
		return 0, nil, shared.NewError(shared.CodeValidationFailed,
			map[string]any{"field": "beforeTaskId", "reason": "only one neighbour"})
	}

	index := len(ordered)
	switch {
	case before != nil:
		found := indexOf(ordered, *before)
		if found < 0 {
			return 0, nil, shared.NewError(shared.CodeTaskNotFound,
				map[string]any{"taskId": before.String()})
		}
		index = found
	case after != nil:
		found := indexOf(ordered, *after)
		if found < 0 {
			return 0, nil, shared.NewError(shared.CodeTaskNotFound,
				map[string]any{"taskId": after.String()})
		}
		index = found + 1
	}

	lower := int32(0)
	if index > 0 {
		lower = ordered[index-1].Position
	}
	upper := lower + 2*PositionGap
	if index < len(ordered) {
		upper = ordered[index].Position
	}

	// Two neighbours with nothing between them, or a scope that has climbed as
	// far as it may: either way it has to be spread out again.
	if upper-lower < 2 || upper > PositionCeiling {
		return rebalance(ordered, index)
	}
	return lower + (upper-lower)/2, nil, nil
}

// rebalance renumbers a scope with a fresh gap between every task, leaving
// the slot at index free for the task being placed.
func rebalance(ordered []Task, index int) (int32, []Placement, error) {
	placements := make([]Placement, 0, len(ordered))

	for i := range ordered {
		slot := i
		if i >= index {
			slot = i + 1
		}
		placements = append(placements, Placement{
			ID:       ordered[i].ID,
			Position: int32(slot+1) * PositionGap,
		})
	}
	return int32(index+1) * PositionGap, placements, nil
}

func indexOf(ordered []Task, id uuid.UUID) int {
	for i := range ordered {
		if ordered[i].ID == id {
			return i
		}
	}
	return -1
}
