// Package graph holds the network the user draws over their tasks.
package graph

import (
	"context"

	"github.com/google/uuid"

	"github.com/moxicom/cursed_matrix/back/internal/app/port"
	"github.com/moxicom/cursed_matrix/back/internal/domain/activity"
	"github.com/moxicom/cursed_matrix/back/internal/domain/link"
	"github.com/moxicom/cursed_matrix/back/internal/domain/shared"
	"github.com/moxicom/cursed_matrix/back/internal/domain/task"
	"github.com/moxicom/cursed_matrix/back/internal/domain/user"
)

type Service struct {
	links  port.LinkRepository
	tasks  port.TaskRepository
	users  port.UserRepository
	events port.ActivityRepository
	tx     port.TxManager
	clock  shared.Clock
}

func NewService(
	links port.LinkRepository,
	tasks port.TaskRepository,
	users port.UserRepository,
	events port.ActivityRepository,
	tx port.TxManager,
	clock shared.Clock,
) *Service {
	return &Service{links: links, tasks: tasks, users: users, events: events, tx: tx, clock: clock}
}

// Links returns every edge the user has drawn.
func (s *Service) Links(ctx context.Context, userID uuid.UUID) ([]link.Link, error) {
	return s.links.List(ctx, userID)
}

// Create joins two tasks.
//
// Both ends are read first, which is also the ownership check: a task that
// belongs to someone else is not found, so a link cannot be used to discover
// that another account holds a given id.
func (s *Service) Create(
	ctx context.Context,
	userID, sourceID, targetID uuid.UUID,
	linkType shared.LinkType,
) (*link.Link, error) {
	var created *link.Link

	err := s.tx.Do(ctx, func(ctx context.Context) error {
		made, err := link.New(uuid.New(), userID, sourceID, targetID, linkType, s.clock.Now().UTC())
		if err != nil {
			return err
		}

		source, err := s.tasks.ByID(ctx, userID, sourceID)
		if err != nil {
			return err
		}
		target, err := s.tasks.ByID(ctx, userID, targetID)
		if err != nil {
			return err
		}
		if parentOf(source, target) || parentOf(target, source) {
			return shared.NewError(shared.CodeDuplicateLink,
				map[string]any{"reason": "PARENT_RELATION"})
		}

		// Counted under the account lock for the same reason tasks are: two
		// requests would otherwise both find the last slot free.
		if err := s.users.LockAccount(ctx, userID); err != nil {
			return err
		}
		if err := s.withinQuota(ctx, userID); err != nil {
			return err
		}

		if err := s.links.Create(ctx, made); err != nil {
			return err
		}

		event, err := activity.New(userID, shared.EventTaskLinked, &sourceID, made.CreatedAt)
		if err != nil {
			return err
		}
		if err := s.events.Record(ctx, event.
			With("targetTaskId", targetID.String()).
			With("type", string(made.Type))); err != nil {
			return err
		}
		if _, err := s.users.ApplyStats(ctx, userID, user.StatsDelta{LinksCreated: 1}); err != nil {
			return err
		}

		created = made
		return nil
	})
	if err != nil {
		return nil, err
	}
	return created, nil
}

// Retype changes what an existing link means.
func (s *Service) Retype(
	ctx context.Context,
	userID, linkID uuid.UUID,
	linkType shared.LinkType,
) (*link.Link, error) {
	var updated *link.Link

	err := s.tx.Do(ctx, func(ctx context.Context) error {
		item, err := s.links.ByID(ctx, userID, linkID)
		if err != nil {
			return err
		}
		if err := item.Retype(linkType); err != nil {
			return err
		}
		// The uniqueness rules are written per type, so the new type may
		// collide with a link that already joins this pair; the index says so.
		if err := s.links.Update(ctx, item); err != nil {
			return err
		}
		updated = item
		return nil
	})
	if err != nil {
		return nil, err
	}
	return updated, nil
}

// Delete breaks a link.
func (s *Service) Delete(ctx context.Context, userID, linkID uuid.UUID) error {
	return s.links.Remove(ctx, userID, linkID)
}

func (s *Service) withinQuota(ctx context.Context, userID uuid.UUID) error {
	// Every account is on the free plan until billing exists; when it does,
	// the plan comes from the account rather than from here.
	limit := user.TaskLinkLimit(shared.PlanFree)
	if limit == user.Unlimited {
		return nil
	}

	held, err := s.links.Count(ctx, userID)
	if err != nil {
		return err
	}
	if held >= limit {
		return shared.NewError(shared.CodeQuotaLimitReached,
			map[string]any{"limit": limit, "resource": "TASK_LINKS"})
	}
	return nil
}

// parentOf reports the system relation, which a user-made link must not
// duplicate: the graph already draws it as its own kind of edge.
func parentOf(parent, child *task.Task) bool {
	return child.ParentID != nil && *child.ParentID == parent.ID
}

// Edge is a line on the canvas. Two things draw as one: the links the user
// made, and the parent relation the server maintains.
type Edge struct {
	ID       string
	SourceID uuid.UUID
	TargetID uuid.UUID
	Kind     string
	Type     shared.LinkType
}

// Snapshot is what the canvas renders.
type Snapshot struct {
	Nodes []task.Node
	Edges []Edge
}

// EdgeKinds distinguish a user-made link from the system relation.
const (
	EdgeLink        = "LINK"
	EdgeParentChild = "PARENT_CHILD"
)

// Snapshot builds the graph for one filter.
//
// Nodes are filtered, edges are not trimmed to them: an edge whose other end
// is filtered out still tells the canvas the node is connected, and the client
// decides how to draw a dangling end. Parent relations are edges here but not
// rows anywhere — they belong to the task, not to the link table.
func (s *Service) Snapshot(ctx context.Context, userID uuid.UUID, filter task.Filter) (*Snapshot, error) {
	nodes, err := s.tasks.ListGraph(ctx, filter)
	if err != nil {
		return nil, err
	}

	links, err := s.links.List(ctx, userID)
	if err != nil {
		return nil, err
	}

	edges := make([]Edge, 0, len(links)+len(nodes))
	for i := range links {
		edges = append(edges, Edge{
			ID:       links[i].ID.String(),
			SourceID: links[i].SourceID,
			TargetID: links[i].TargetID,
			Kind:     EdgeLink,
			Type:     links[i].Type,
		})
	}
	for i := range nodes {
		if nodes[i].ParentID == nil {
			continue
		}
		edges = append(edges, Edge{
			// Derived, not stored, so it needs an id the client can key on
			// that cannot collide with a link's.
			ID:       "p-" + nodes[i].ID.String(),
			SourceID: nodes[i].ID,
			TargetID: *nodes[i].ParentID,
			Kind:     EdgeParentChild,
		})
	}

	return &Snapshot{Nodes: nodes, Edges: edges}, nil
}
