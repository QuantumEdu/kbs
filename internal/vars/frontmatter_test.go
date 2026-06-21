package vars

import (
	"testing"
)

func TestParseFrontmatterNoFrontmatter(t *testing.T) {
	content := "# Just body\n\nNo frontmatter here."
	fm, body, err := ParseFrontmatter(content, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fm.Description != "" {
		t.Errorf("Description = %q, want empty", fm.Description)
	}
	if body != content {
		t.Errorf("body = %q, want %q", body, content)
	}
}

func TestParseFrontmatterBasic(t *testing.T) {
	content := `---
description: "A test memory file"
tags:
  - go
  - cli
created: "2026-06-01"
---
# Hello World

This is the body.
`
	fm, body, err := ParseFrontmatter(content, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fm.Description != "A test memory file" {
		t.Errorf("Description = %q, want %q", fm.Description, "A test memory file")
	}
	if len(fm.Tags) != 2 || fm.Tags[0] != "go" || fm.Tags[1] != "cli" {
		t.Errorf("Tags = %v, want [go cli]", fm.Tags)
	}
	if fm.Created != "2026-06-01" {
		t.Errorf("Created = %q, want 2026-06-01", fm.Created)
	}
	expectedBody := "# Hello World\n\nThis is the body.\n"
	if body != expectedBody {
		t.Errorf("body = %q, want %q", body, expectedBody)
	}
}

func TestParseFrontmatterInlineTags(t *testing.T) {
	content := `---
description: "Inline tags test"
tags: python, data-science
---
Body
`
	fm, _, err := ParseFrontmatter(content, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(fm.Tags) != 2 || fm.Tags[0] != "python" || fm.Tags[1] != "data-science" {
		t.Errorf("Tags = %v, want [python data-science]", fm.Tags)
	}
}

func TestParseFrontmatterNoBody(t *testing.T) {
	content := "---\ndescription: Just frontmatter\n---"
	fm, body, err := ParseFrontmatter(content, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fm.Description != "Just frontmatter" {
		t.Errorf("Description = %q, want %q", fm.Description, "Just frontmatter")
	}
	if body != "" {
		t.Errorf("body = %q, want empty", body)
	}
}

func TestParseFrontmatterMalformed(t *testing.T) {
	// No closing ---
	content := "---\ndescription: Broken\n"
	_, _, err := ParseFrontmatter(content, false)
	if err == nil {
		t.Fatal("expected error for unclosed frontmatter")
	}
}

func TestParseFrontmatterWikilinks(t *testing.T) {
	content := `---
description: "With links"
---
# Entry

See [[other-entry]] and [[another-file]] for details.
`
	fm, _, err := ParseFrontmatter(content, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(fm.Links) != 2 {
		t.Fatalf("expected 2 links, got %d: %v", len(fm.Links), fm.Links)
	}
	if fm.Links[0] != "other-entry" {
		t.Errorf("Links[0] = %q, want 'other-entry'", fm.Links[0])
	}
}

func TestParseFrontmatterWikilinksDisabled(t *testing.T) {
	content := `---
description: "No link parsing"
---
See [[other-entry]].
`
	fm, _, err := ParseFrontmatter(content, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(fm.Links) != 0 {
		t.Errorf("expected 0 links (disabled), got %d", len(fm.Links))
	}
}

func TestFirstHeading(t *testing.T) {
	body := "Some text\n# Main Title\nmore text"
	h := FirstHeading(body)
	if h != "Main Title" {
		t.Errorf("FirstHeading = %q, want 'Main Title'", h)
	}
}

func TestFirstHeadingEmpty(t *testing.T) {
	body := "No heading here.\nJust text."
	h := FirstHeading(body)
	if h != "" {
		t.Errorf("FirstHeading = %q, want empty", h)
	}
}

func TestFirstParagraph(t *testing.T) {
	body := "# Title\n\nThis is the first paragraph. It has some content.\n\nThis is the second paragraph."
	p := FirstParagraph(body)
	if p != "This is the first paragraph. It has some content." {
		t.Errorf("FirstParagraph = %q, want first paragraph", p)
	}
}

func TestFirstParagraphNoContent(t *testing.T) {
	p := FirstParagraph("")
	if p != "" {
		t.Errorf("FirstParagraph = %q, want empty", p)
	}
}

func TestFirstParagraphTruncates(t *testing.T) {
	long := ""
	for i := 0; i < 100; i++ {
		long += "word "
	}
	body := "# Title\n\n" + long
	p := FirstParagraph(body)
	if len(p) > 600 {
		t.Errorf("FirstParagraph too long: %d chars (expected ~500)", len(p))
	}
}

func TestParseFrontmatterWithUpdated(t *testing.T) {
	content := `---
description: "Has updated"
created: "2026-01-01"
updated: "2026-06-15"
---
Body
`
	fm, _, err := ParseFrontmatter(content, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fm.Updated != "2026-06-15" {
		t.Errorf("Updated = %q, want '2026-06-15'", fm.Updated)
	}
}

func TestParseFrontmatterEmptyTags(t *testing.T) {
	content := `---
description: "No tags"
---
Body
`
	fm, _, err := ParseFrontmatter(content, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fm.Tags == nil {
		t.Error("Tags should be non-nil empty slice")
	}
	if len(fm.Tags) != 0 {
		t.Errorf("Tags = %v, want empty", fm.Tags)
	}
}

func TestParseFrontmatterWikilinkWithPipe(t *testing.T) {
	content := `---
description: "Piped link"
---
See [[target-file|display text]] for details.
`
	fm, _, err := ParseFrontmatter(content, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(fm.Links) != 1 {
		t.Fatalf("expected 1 link, got %d", len(fm.Links))
	}
	if fm.Links[0] != "target-file" {
		t.Errorf("Links[0] = %q, want 'target-file'", fm.Links[0])
	}
}
