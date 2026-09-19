// Package link holds the user-made relation between two tasks.
package link

import (
	"time"

	"github.com/google/uuid"

	"github.com/moxicom/cursed_matrix/back/internal/domain/shared"
)

// Link joins two tasks of one user.
//
// A parent-child relation is not one of these: that one is a system relation
// the server maintains, and this one is the user's to make and break.
type Link struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	SourceID  uuid.UUID
	TargetID  uuid.UUID
	Type      shared.LinkType
	CreatedAt time.Time
}

// Directed reports whether the order of the two ends carries meaning.
//
// BLOCKS and DEPENDS_ON say something about which end is which; RELATED and
// CONNECTED_TO do not, so A→B and B→A are the same link and the uniqueness
// rule normalises the pair.
func Directed(linkType shared.LinkType) bool {
	return linkType == shared.LinkBlocks || linkType == shared.LinkDependsOn
}

// New makes a link between two distinct tasks.
func New(id, userID, sourceID, targetID uuid.UUID, linkType shared.LinkType, now time.Time) (*Link, error) {
	if linkType == "" {
		linkType = shared.LinkRelated
	}
	if !linkType.Valid() {
		return nil, shared.NewError(shared.CodeValidationFailed,
			map[string]any{"field": "type", "value": string(linkType)})
	}
	if sourceID == uuid.Nil || targetID == uuid.Nil || userID == uuid.Nil || id == uuid.Nil {
		return nil, shared.NewError(shared.CodeValidationFailed, map[string]any{"field": "id"})
	}
	// A task related to itself says nothing and would draw a loop in the
	// graph that no filter can make sense of.
	if sourceID == targetID {
		return nil, shared.NewError(shared.CodeSelfLink,
			map[string]any{"taskId": sourceID.String()})
	}

	return &Link{
		ID: id, UserID: userID,
		SourceID: sourceID, TargetID: targetID,
		Type:      linkType,
		CreatedAt: now,
	}, nil
}

// Retype changes what the link means, keeping its ends.
func (l *Link) Retype(linkType shared.LinkType) error {
	if !linkType.Valid() {
		return shared.NewError(shared.CodeValidationFailed,
			map[string]any{"field": "type", "value": string(linkType)})
	}
	l.Type = linkType
	return nil
}

// SameEnds reports whether two links join the same pair, accounting for
// direction: for an undirected type the pair is the same either way round.
func (l *Link) SameEnds(other *Link) bool {
	if l.Type != other.Type {
		return false
	}
	if Directed(l.Type) {
		return l.SourceID == other.SourceID && l.TargetID == other.TargetID
	}
	return (l.SourceID == other.SourceID && l.TargetID == other.TargetID) ||
		(l.SourceID == other.TargetID && l.TargetID == other.SourceID)
}
