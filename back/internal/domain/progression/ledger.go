package progression

import (
	"time"

	"github.com/google/uuid"

	"github.com/moxicom/cursed_matrix/back/internal/domain/shared"
)

// Entry is one movement of XP.
//
// The ledger is append-only: a grant that turns out not to have been earned is
// answered by a second entry with the opposite sign, never by editing or
// removing the first. That is what keeps a leaderboard recomputable from the
// ledger alone.
type Entry struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	TaskID    *uuid.UUID
	Amount    int32
	Source    shared.XPSource
	GrantSeq  int
	CreatedAt time.Time
	Metadata  map[string]any
}

// Grant records XP earned by completing a task.
//
// The quadrant and the configuration version travel with the entry: the amount
// was decided by rules that may change, and an audit later has to be able to
// see which ones applied.
func Grant(
	userID, taskID uuid.UUID,
	amount int32,
	isSubtask bool,
	quadrant shared.Quadrant,
	configVersion, grantSeq int,
	at time.Time,
) (*Entry, error) {
	if amount <= 0 {
		return nil, shared.NewError(shared.CodeValidationFailed,
			map[string]any{"field": "amount", "value": amount})
	}
	if grantSeq < 1 {
		return nil, shared.NewError(shared.CodeValidationFailed,
			map[string]any{"field": "grantSeq", "value": grantSeq})
	}

	source := shared.XPTaskCompleted
	if isSubtask {
		source = shared.XPSubtaskCompleted
	}

	id := taskID
	return &Entry{
		ID:        uuid.New(),
		UserID:    userID,
		TaskID:    &id,
		Amount:    amount,
		Source:    source,
		GrantSeq:  grantSeq,
		CreatedAt: at,
		Metadata: map[string]any{
			"quadrant":      string(quadrant),
			"configVersion": configVersion,
		},
	}, nil
}

// Revoke records the compensating entry for XP that is being taken back.
//
// The source says why — a task returned to work, or one removed — because an
// audit of where a user's XP went cannot read that from the amount alone.
func Revoke(userID, taskID uuid.UUID, granted int32, source shared.XPSource, at time.Time) (*Entry, error) {
	if granted <= 0 {
		return nil, shared.NewError(shared.CodeValidationFailed,
			map[string]any{"field": "amount", "value": granted})
	}
	if source != shared.XPTaskReopened && source != shared.XPTaskDeleted {
		return nil, shared.NewError(shared.CodeValidationFailed,
			map[string]any{"field": "source", "value": string(source)})
	}

	id := taskID
	return &Entry{
		ID:        uuid.New(),
		UserID:    userID,
		TaskID:    &id,
		Amount:    -granted,
		Source:    source,
		GrantSeq:  1,
		CreatedAt: at,
		Metadata:  map[string]any{"revoked": granted},
	}, nil
}
