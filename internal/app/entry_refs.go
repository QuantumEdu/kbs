package app

import (
	"context"
	"fmt"

	"github.com/quantum-6/skillvault/internal/db"
	"github.com/quantum-6/skillvault/internal/domain"
)

// EntryRefService handles cycle-safe graph operations on entry_links.
type EntryRefService struct {
	store      db.EntryLinkStore
	entryStore db.EntryStore
}

func NewEntryRefService(store db.EntryLinkStore, entryStore db.EntryStore) *EntryRefService {
	return &EntryRefService{
		store:      store,
		entryStore: entryStore,
	}
}

// Input for creating a new ref.
type AddRefInput struct {
	SourceID string
	TargetID string
	RefType  string
	Label    string
}

// SaveRef creates a graph edge with cycle detection for cycle-prone relation types.
func (s *EntryRefService) SaveRef(ctx context.Context, input AddRefInput) (*domain.EntryLink, error) {
	// Validate relation type
	if err := domain.ValidateRelationType(input.RefType); err != nil {
		return nil, err
	}

	// Validate source and target exist
	if _, err := s.entryStore.Get(ctx, input.SourceID, true); err != nil {
		return nil, fmt.Errorf("source entry %q not found: %w", input.SourceID, err)
	}
	if _, err := s.entryStore.Get(ctx, input.TargetID, true); err != nil {
		return nil, fmt.Errorf("target entry %q not found: %w", input.TargetID, err)
	}

	rt := domain.RelationType(input.RefType)

	// Cycle detection for cycle-prone types
	if rt.CycleProne() {
		reachable, err := s.store.ReachableRefs(ctx, input.TargetID, input.RefType, 10)
		if err != nil {
			return nil, fmt.Errorf("cycle detection failed: %w", err)
		}
		for _, n := range reachable {
			if n.EntryID == input.SourceID {
				return nil, fmt.Errorf("cycle_detected: %s is already reachable from %s via %s refs (depth=%d)",
					input.SourceID, input.TargetID, input.RefType, n.Depth)
			}
		}
	}

	link := domain.EntryLink{
		FromEntryID:  input.SourceID,
		ToEntryID:    input.TargetID,
		RelationType: rt,
		Label:        input.Label,
		Active:       true,
	}

	if err := s.store.Save(ctx, link); err != nil {
		return nil, fmt.Errorf("save ref: %w", err)
	}

	return &link, nil
}

// ListRefs returns refs matching the given filters.
type ListRefsInput struct {
	SourceID        *string
	TargetID        *string
	RefType         *string
	IncludeArchived bool
}

func (s *EntryRefService) ListRefs(ctx context.Context, input ListRefsInput) ([]domain.EntryLink, error) {
	return s.store.ListRefs(ctx, db.EntryLinkFilter{
		SourceID:        input.SourceID,
		TargetID:        input.TargetID,
		RefType:         input.RefType,
		IncludeArchived: input.IncludeArchived,
	})
}

// RemoveRef soft-deletes a ref.
func (s *EntryRefService) RemoveRef(ctx context.Context, sourceID, targetID, refType string) error {
	if err := domain.ValidateRelationType(refType); err != nil {
		return err
	}
	return s.store.RemoveRef(ctx, sourceID, targetID, refType)
}

// GetEntryGraph returns the graph of nodes/edges reachable from the given entry.
type GetGraphInput struct {
	EntryID   string
	RefTypes  []string
	Direction string
	MaxDepth  int
}

type GraphResult struct {
	Nodes []db.EntryLinkNode
	Edges []domain.EntryLink
}

func (s *EntryRefService) GetEntryGraph(ctx context.Context, input GetGraphInput) (*GraphResult, error) {
	if input.MaxDepth <= 0 {
		input.MaxDepth = 3
	}
	if input.MaxDepth > 10 {
		input.MaxDepth = 10
	}
	if input.Direction == "" {
		input.Direction = "both"
	}
	if input.Direction != "outgoing" && input.Direction != "incoming" && input.Direction != "both" {
		return nil, fmt.Errorf("invalid direction: %q (expected: outgoing, incoming, both)", input.Direction)
	}

	nodes, edges, err := s.store.GetEntryGraph(ctx, input.EntryID, input.RefTypes, input.Direction, input.MaxDepth)
	if err != nil {
		return nil, err
	}

	return &GraphResult{Nodes: nodes, Edges: edges}, nil
}
