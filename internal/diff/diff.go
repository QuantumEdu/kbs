package diff

import (
	"strings"
)

// DiffLine represents a single line in a unified diff.
type DiffLine struct {
	Type    byte   // ' ' = context, '+' = added, '-' = removed
	Line    string // The line content (without prefix)
	OldLine int    // Line number in the old text (0 for added lines)
	NewLine int    // Line number in the new text (0 for removed lines)
}

// UnifiedDiff computes the line-based diff between oldText and newText
// using the LCS (Longest Common Subsequence) algorithm.
// Returns the diff as a slice of DiffLine entries.
func UnifiedDiff(oldText, newText string) []DiffLine {
	oldLines := splitLines(oldText)
	newLines := splitLines(newText)

	table := buildLCSTable(oldLines, newLines)
	ops := backtrack(table, oldLines, newLines)
	return toDiffLines(ops, oldLines, newLines)
}

// FormatUnifiedDiff formats a slice of DiffLine entries as a unified diff string.
// Returns an empty string if there are no changes.
func FormatUnifiedDiff(lines []DiffLine) string {
	if len(lines) == 0 {
		return ""
	}

	hasChanges := false
	for _, l := range lines {
		if l.Type != ' ' {
			hasChanges = true
			break
		}
	}
	if !hasChanges {
		return "No changes."
	}

	var b strings.Builder

	// Find first and last changed line ranges to produce the hunk header.
	var oldStart, oldEnd, newStart, newEnd int
	first := true
	for _, l := range lines {
		if first {
			if l.OldLine > 0 {
				oldStart = l.OldLine
			} else if l.NewLine > 0 {
				oldStart = l.NewLine
			}
			if l.NewLine > 0 {
				newStart = l.NewLine
			} else if l.OldLine > 0 {
				newStart = l.OldLine
			}
			first = false
		}
		if l.OldLine > 0 {
			oldEnd = l.OldLine
		}
		if l.NewLine > 0 {
			newEnd = l.NewLine
		}
	}

	oldLen := oldEnd - oldStart + 1
	newLen := newEnd - newStart + 1
	b.WriteString("--- a\n")
	b.WriteString("+++ b\n")
	b.WriteString(formatHunkHeader(oldStart, oldLen, newStart, newLen))
	b.WriteString("\n")

	for _, l := range lines {
		b.WriteByte(l.Type)
		b.WriteString(l.Line)
		b.WriteByte('\n')
	}

	return b.String()
}

// splitLines splits text into lines. An empty string returns an empty slice.
func splitLines(text string) []string {
	if text == "" {
		return nil
	}
	lines := strings.Split(text, "\n")
	return lines
}

// buildLCSTable builds the LCS dynamic programming table.
func buildLCSTable(a, b []string) [][]int {
	m, n := len(a), len(b)
	table := make([][]int, m+1)
	for i := range table {
		table[i] = make([]int, n+1)
	}
	for i := 1; i <= m; i++ {
		for j := 1; j <= n; j++ {
			if a[i-1] == b[j-1] {
				table[i][j] = table[i-1][j-1] + 1
			} else if table[i-1][j] >= table[i][j-1] {
				table[i][j] = table[i-1][j]
			} else {
				table[i][j] = table[i][j-1]
			}
		}
	}
	return table
}

// opKind represents a diff edit operation.
type opKind byte

const (
	opContext opKind = ' '
	opAdd     opKind = '+'
	opRemove  opKind = '-'
)

// diffOp is a single edit operation produced by LCS backtracking.
type diffOp struct {
	kind    opKind
	oldLine int // 1-based line in old text (0 for adds)
	newLine int // 1-based line in new text (0 for removes)
	text    string
}

// backtrack walks the LCS table to produce a forward sequence of edit operations.
func backtrack(table [][]int, oldLines, newLines []string) []diffOp {
	var ops []diffOp
	i, j := len(oldLines), len(newLines)
	for i > 0 || j > 0 {
		if i > 0 && j > 0 && oldLines[i-1] == newLines[j-1] {
			ops = append(ops, diffOp{kind: opContext, oldLine: i, newLine: j, text: oldLines[i-1]})
			i--
			j--
		} else if j > 0 && (i == 0 || table[i][j-1] >= table[i-1][j]) {
			ops = append(ops, diffOp{kind: opAdd, oldLine: 0, newLine: j, text: newLines[j-1]})
			j--
		} else {
			ops = append(ops, diffOp{kind: opRemove, oldLine: i, newLine: 0, text: oldLines[i-1]})
			i--
		}
	}
	// Reverse to get forward order.
	for l, r := 0, len(ops)-1; l < r; l, r = l+1, r-1 {
		ops[l], ops[r] = ops[r], ops[l]
	}
	return ops
}

// toDiffLines converts edit operations into DiffLine entries.
func toDiffLines(ops []diffOp, oldLines, newLines []string) []DiffLine {
	var lines []DiffLine
	for _, op := range ops {
		lines = append(lines, DiffLine{
			Type:    byte(op.kind),
			Line:    op.text,
			OldLine: op.oldLine,
			NewLine: op.newLine,
		})
	}
	return lines
}

// formatHunkHeader produces a unified diff hunk header like "@@ -1,3 +1,4 @@".
func formatHunkHeader(oldStart, oldLen, newStart, newLen int) string {
	var b strings.Builder
	b.WriteString("@@ -")
	b.WriteString(itoa(oldStart))
	if oldLen != 1 {
		b.WriteByte(',')
		b.WriteString(itoa(oldLen))
	}
	b.WriteString(" +")
	b.WriteString(itoa(newStart))
	if newLen != 1 {
		b.WriteByte(',')
		b.WriteString(itoa(newLen))
	}
	b.WriteString(" @@")
	return b.String()
}

// itoa is a minimal int-to-string for non-negative integers (avoids fmt import).
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}
