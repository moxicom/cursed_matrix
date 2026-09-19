package board

import (
	"context"

	"github.com/google/uuid"

	"github.com/moxicom/cursed_matrix/back/internal/domain/tag"
)

// Tags returns the user's labels with how many tasks carry each.
func (s *Service) Tags(ctx context.Context, userID uuid.UUID) ([]tag.Tag, error) {
	return s.tags.List(ctx, userID)
}

// AttachTag puts a label on a task, creating the label if it is new.
func (s *Service) AttachTag(ctx context.Context, userID, taskID uuid.UUID, name string) (*tag.Tag, error) {
	clean, err := tag.NormalizeName(name)
	if err != nil {
		return nil, err
	}

	var attached *tag.Tag
	err = s.tx.Do(ctx, func(ctx context.Context) error {
		// Reading the task is the ownership check: someone else's task must
		// not be taggable, and must not be distinguishable from a missing one.
		if _, err := s.tasks.ByID(ctx, userID, taskID); err != nil {
			return err
		}

		label, err := s.tags.Upsert(ctx, userID, clean)
		if err != nil {
			return err
		}
		if err := s.tags.Attach(ctx, userID, taskID, label.ID); err != nil {
			return err
		}
		attached = label
		return nil
	})
	if err != nil {
		return nil, err
	}
	return attached, nil
}

// DetachTag takes a label off a task and forgets labels left on nothing.
func (s *Service) DetachTag(ctx context.Context, userID, taskID, tagID uuid.UUID) error {
	return s.tx.Do(ctx, func(ctx context.Context) error {
		if err := s.tags.Detach(ctx, userID, taskID, tagID); err != nil {
			return err
		}
		return s.tags.DeleteOrphans(ctx, userID)
	})
}
