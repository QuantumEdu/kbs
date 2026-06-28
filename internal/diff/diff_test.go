package diff

import (
	"strings"
	"testing"
)

func TestUnifiedDiff_Identity(t *testing.T) {
	text := "line 1\nline 2\nline 3"
	lines := UnifiedDiff(text, text)
	for _, l := range lines {
		if l.Type != ' ' {
			t.Errorf("expected all context lines for identical input, got type=%q at line %q", l.Type, l.Line)
		}
	}
}

func TestUnifiedDiff_SingleInsert(t *testing.T) {
	oldText := "line 1\nline 2"
	newText := "line 1\nline 1.5\nline 2"

	lines := UnifiedDiff(oldText, newText)

	hasAdd := false
	for _, l := range lines {
		if l.Type == '+' {
			hasAdd = true
			if l.Line != "line 1.5" {
				t.Errorf("expected added line %q, got %q", "line 1.5", l.Line)
			}
		}
	}
	if !hasAdd {
		t.Error("expected at least one added line")
	}
}

func TestUnifiedDiff_SingleDelete(t *testing.T) {
	oldText := "line 1\nline 2\nline 3"
	newText := "line 1\nline 3"

	lines := UnifiedDiff(oldText, newText)

	hasRemove := false
	for _, l := range lines {
		if l.Type == '-' {
			hasRemove = true
			if l.Line != "line 2" {
				t.Errorf("expected removed line %q, got %q", "line 2", l.Line)
			}
		}
	}
	if !hasRemove {
		t.Error("expected at least one removed line")
	}
}

func TestUnifiedDiff_SingleEdit(t *testing.T) {
	oldText := "line 1\nline 2\nline 3"
	newText := "line 1\nline 2 edited\nline 3"

	lines := UnifiedDiff(oldText, newText)

	var ops []byte
	for _, l := range lines {
		ops = append(ops, l.Type)
	}
	opsStr := string(ops)
	if !strings.Contains(opsStr, "-") {
		t.Error("expected a removed line (old line 2)")
	}
	if !strings.Contains(opsStr, "+") {
		t.Error("expected an added line (new line 2)")
	}
}

func TestUnifiedDiff_MultiEditInsertDelete(t *testing.T) {
	oldText := "line 1\nline 2\nline 3\nline 4"
	newText := "line 1\nline 2 edited\nline 4\nline 5"

	lines := UnifiedDiff(oldText, newText)

	var ops []byte
	for _, l := range lines {
		ops = append(ops, l.Type)
	}
	opsStr := string(ops)
	if !strings.Contains(opsStr, "-") {
		t.Error("expected at least one removal")
	}
	if !strings.Contains(opsStr, "+") {
		t.Error("expected at least one addition")
	}
}

func TestUnifiedDiff_EmptyOld(t *testing.T) {
	lines := UnifiedDiff("", "line 1\nline 2")
	diff := FormatUnifiedDiff(lines)
	if !strings.Contains(diff, "+line 1") {
		t.Errorf("expected added lines in diff, got:\n%s", diff)
	}
	if !strings.Contains(diff, "+line 2") {
		t.Errorf("expected added lines in diff, got:\n%s", diff)
	}
}

func TestUnifiedDiff_EmptyNew(t *testing.T) {
	lines := UnifiedDiff("line 1\nline 2", "")
	diff := FormatUnifiedDiff(lines)
	if !strings.Contains(diff, "-line 1") {
		t.Errorf("expected removed lines in diff, got:\n%s", diff)
	}
	if !strings.Contains(diff, "-line 2") {
		t.Errorf("expected removed lines in diff, got:\n%s", diff)
	}
}

func TestUnifiedDiff_BothEmpty(t *testing.T) {
	lines := UnifiedDiff("", "")
	if len(lines) > 0 {
		t.Errorf("expected no diff lines for empty input, got %d", len(lines))
	}
}

func TestUnifiedDiff_SingleLineIdentical(t *testing.T) {
	lines := UnifiedDiff("hello", "hello")
	if len(lines) != 1 || lines[0].Type != ' ' {
		t.Errorf("expected single context line, got %+v", lines)
	}
}

func TestUnifiedDiff_SingleLineEdit(t *testing.T) {
	lines := UnifiedDiff("hello", "world")
	var ops []byte
	for _, l := range lines {
		ops = append(ops, l.Type)
	}
	if string(ops) != "-+" {
		t.Errorf("expected one removal and one addition, got %q", string(ops))
	}
}

func TestFormatUnifiedDiff_NoChanges(t *testing.T) {
	lines := []DiffLine{
		{Type: ' ', Line: "line 1", OldLine: 1, NewLine: 1},
		{Type: ' ', Line: "line 2", OldLine: 2, NewLine: 2},
	}
	result := FormatUnifiedDiff(lines)
	if result != "No changes." {
		t.Errorf("expected 'No changes.', got %q", result)
	}
}

func TestFormatUnifiedDiff_HasChanges(t *testing.T) {
	lines := []DiffLine{
		{Type: ' ', Line: "line 1", OldLine: 1, NewLine: 1},
		{Type: '-', Line: "line 2 old", OldLine: 2, NewLine: 0},
		{Type: '+', Line: "line 2 new", OldLine: 0, NewLine: 2},
		{Type: ' ', Line: "line 3", OldLine: 3, NewLine: 3},
	}
	result := FormatUnifiedDiff(lines)

	if !strings.Contains(result, "--- a") {
		t.Error("expected diff header")
	}
	if !strings.Contains(result, "+++ b") {
		t.Error("expected diff header")
	}
	if !strings.Contains(result, "@@") {
		t.Error("expected hunk header")
	}
	if !strings.Contains(result, "-line 2 old") {
		t.Error("expected removed line")
	}
	if !strings.Contains(result, "+line 2 new") {
		t.Error("expected added line")
	}
}

func TestUnifiedDiff_EmptyLines(t *testing.T) {
	oldText := "line 1\n\nline 3"
	newText := "line 1\nline 2\nline 3"

	lines := UnifiedDiff(oldText, newText)
	diff := FormatUnifiedDiff(lines)

	if !strings.Contains(diff, "-") {
		t.Errorf("expected removal (empty line), got:\n%s", diff)
	}
	if !strings.Contains(diff, "+line 2") {
		t.Errorf("expected addition, got:\n%s", diff)
	}
}
