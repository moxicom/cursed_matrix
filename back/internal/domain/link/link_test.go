package link_test

import (
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/moxicom/cursed_matrix/back/internal/domain/link"
	"github.com/moxicom/cursed_matrix/back/internal/domain/shared"
)

func TestNew(t *testing.T) {
	userID, source, target := uuid.New(), uuid.New(), uuid.New()
	now := time.Date(2026, 9, 19, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name     string
		source   uuid.UUID
		target   uuid.UUID
		linkType shared.LinkType
		wantType shared.LinkType
		wantErr  shared.ErrorCode
	}{
		{name: "an ordinary link", source: source, target: target, linkType: shared.LinkBlocks, wantType: shared.LinkBlocks},
		{name: "no type means related", source: source, target: target, wantType: shared.LinkRelated},
		{
			name:     "an unknown type is refused",
			source:   source,
			target:   target,
			linkType: shared.LinkType("SUPERSEDES"),
			wantErr:  shared.CodeValidationFailed,
		},
		{
			name:    "a task cannot be linked to itself",
			source:  source,
			target:  source,
			wantErr: shared.CodeSelfLink,
		},
		{
			name:    "an absent end is refused",
			source:  source,
			target:  uuid.Nil,
			wantErr: shared.CodeValidationFailed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			made, err := link.New(uuid.New(), userID, tt.source, tt.target, tt.linkType, now)

			if tt.wantErr != "" {
				if shared.CodeOf(err) != tt.wantErr {
					t.Fatalf("err = %v, want %s", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("New: %v", err)
			}
			if made.Type != tt.wantType {
				t.Errorf("type = %q, want %q", made.Type, tt.wantType)
			}
		})
	}
}

func TestSameEnds(t *testing.T) {
	a, b := uuid.New(), uuid.New()
	userID := uuid.New()
	now := time.Now().UTC()

	make := func(source, target uuid.UUID, linkType shared.LinkType) *link.Link {
		made, err := link.New(uuid.New(), userID, source, target, linkType, now)
		if err != nil {
			t.Fatalf("New: %v", err)
		}
		return made
	}

	tests := []struct {
		name string
		one  *link.Link
		two  *link.Link
		want bool
	}{
		{
			name: "the same undirected pair either way round",
			one:  make(a, b, shared.LinkRelated),
			two:  make(b, a, shared.LinkRelated),
			want: true,
		},
		{
			name: "a directed pair reversed is a different link",
			one:  make(a, b, shared.LinkBlocks),
			two:  make(b, a, shared.LinkBlocks),
		},
		{
			name: "the same directed pair",
			one:  make(a, b, shared.LinkDependsOn),
			two:  make(a, b, shared.LinkDependsOn),
			want: true,
		},
		{
			name: "the same ends with different meanings",
			one:  make(a, b, shared.LinkRelated),
			two:  make(a, b, shared.LinkConnectedTo),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.one.SameEnds(tt.two); got != tt.want {
				t.Errorf("SameEnds = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDirected(t *testing.T) {
	tests := []struct {
		linkType shared.LinkType
		want     bool
	}{
		{linkType: shared.LinkBlocks, want: true},
		{linkType: shared.LinkDependsOn, want: true},
		{linkType: shared.LinkRelated},
		{linkType: shared.LinkConnectedTo},
	}

	for _, tt := range tests {
		t.Run(string(tt.linkType), func(t *testing.T) {
			if got := link.Directed(tt.linkType); got != tt.want {
				t.Errorf("Directed(%s) = %v, want %v", tt.linkType, got, tt.want)
			}
		})
	}
}
