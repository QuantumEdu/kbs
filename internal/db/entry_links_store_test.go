package db

import (
	"context"
	"testing"

	"github.com/quantum-6/skillvault/internal/domain"
)

func setupLinkStore(t *testing.T) (EntryStore, EntryLinkStore, func()) {
	t.Helper()
	db, err := OpenDB(":memory:")
	if err != nil {
		t.Fatalf("OpenDB failed: %v", err)
	}
	if err := RunMigrations(db); err != nil {
		db.Close()
		t.Fatalf("RunMigrations failed: %v", err)
	}
	entryStore := &sqliteEntryStore{db: db}
	linkStore := &sqliteEntryLinkStore{db: db}
	cleanup := func() { db.Close() }
	return entryStore, linkStore, cleanup
}

func TestSaveAndGetLinks(t *testing.T) {
	estore, lstore, cleanup := setupLinkStore(t)
	defer cleanup()
	ctx := context.Background()

	for _, e := range []domain.Entry{
		{ID: "e1", Title: "E1", Slug: "e1", Type: domain.EntryTypeSkill, BodyOptional: "C1", Status: domain.StatusActive},
		{ID: "e2", Title: "E2", Slug: "e2", Type: domain.EntryTypePrompt, BodyOptional: "C2", Status: domain.StatusActive},
	} {
		if err := estore.Save(ctx, e, nil); err != nil {
			t.Fatalf("Save %s failed: %v", e.ID, err)
		}
	}

	link := domain.EntryLink{
		FromEntryID:  "e1",
		ToEntryID:    "e2",
		RelationType: domain.RelationReferences,
	}
	if err := lstore.Save(ctx, link); err != nil {
		t.Fatalf("Save link failed: %v", err)
	}

	links, err := lstore.GetLinks(ctx, "e1")
	if err != nil {
		t.Fatalf("GetLinks failed: %v", err)
	}
	if len(links) != 1 {
		t.Fatalf("expected 1 link, got %d", len(links))
	}
	if links[0].RelationType != domain.RelationReferences {
		t.Errorf("RelationType = %q, want 'references'", links[0].RelationType)
	}
}

func TestGetLinksByType(t *testing.T) {
	estore, lstore, cleanup := setupLinkStore(t)
	defer cleanup()
	ctx := context.Background()

	for _, e := range []domain.Entry{
		{ID: "e1", Title: "E1", Slug: "e1", Type: domain.EntryTypeSkill, BodyOptional: "C1", Status: domain.StatusActive},
		{ID: "e2", Title: "E2", Slug: "e2", Type: domain.EntryTypePrompt, BodyOptional: "C2", Status: domain.StatusActive},
		{ID: "e3", Title: "E3", Slug: "e3", Type: domain.EntryTypeReference, BodyOptional: "C3", Status: domain.StatusActive},
	} {
		if err := estore.Save(ctx, e, nil); err != nil {
			t.Fatalf("Save %s failed: %v", e.ID, err)
		}
	}

	for _, l := range []domain.EntryLink{
		{FromEntryID: "e1", ToEntryID: "e2", RelationType: domain.RelationReferences},
		{FromEntryID: "e1", ToEntryID: "e3", RelationType: domain.RelationRelatedTo},
	} {
		if err := lstore.Save(ctx, l); err != nil {
			t.Fatalf("Save link failed: %v", err)
		}
	}

	refs, err := lstore.GetLinksByType(ctx, "e1", "references")
	if err != nil {
		t.Fatalf("GetLinksByType failed: %v", err)
	}
	if len(refs) != 1 {
		t.Errorf("expected 1 reference link, got %d", len(refs))
	}

	related, err := lstore.GetLinksByType(ctx, "e1", "related_to")
	if err != nil {
		t.Fatalf("GetLinksByType for related_to failed: %v", err)
	}
	if len(related) != 1 {
		t.Errorf("expected 1 related_to link, got %d", len(related))
	}
}

func TestSaveLinkDeduplicates(t *testing.T) {
	estore, lstore, cleanup := setupLinkStore(t)
	defer cleanup()
	ctx := context.Background()

	for _, e := range []domain.Entry{
		{ID: "e1", Title: "E1", Slug: "e1", Type: domain.EntryTypeSkill, BodyOptional: "C1", Status: domain.StatusActive},
		{ID: "e2", Title: "E2", Slug: "e2", Type: domain.EntryTypePrompt, BodyOptional: "C2", Status: domain.StatusActive},
	} {
		if err := estore.Save(ctx, e, nil); err != nil {
			t.Fatalf("Save %s failed: %v", e.ID, err)
		}
	}

	link := domain.EntryLink{FromEntryID: "e1", ToEntryID: "e2", RelationType: domain.RelationReferences}
	if err := lstore.Save(ctx, link); err != nil {
		t.Fatalf("first Save link failed: %v", err)
	}
	if err := lstore.Save(ctx, link); err != nil {
		t.Fatalf("second Save link should succeed (no-op): %v", err)
	}

	links, err := lstore.GetLinks(ctx, "e1")
	if err != nil {
		t.Fatalf("GetLinks failed: %v", err)
	}
	if len(links) != 1 {
		t.Errorf("expected 1 link after dedup, got %d", len(links))
	}
}
