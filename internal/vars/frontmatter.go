package vars

import (
	"errors"
	"regexp"
	"strings"
)

// Frontmatter holds parsed YAML frontmatter from a pi-memory-md file.
type Frontmatter struct {
	Description string
	Tags        []string
	Created     string
	Updated     string
	Links       []string // from [[wikilinks]] when enabled
}

// wikilinkRe matches [[target-slug]] or [[target-slug|display text]]
var wikilinkRe = regexp.MustCompile(`\[\[([^\]|]+)(?:\|[^\]]+)?\]\]`)

// ParseFrontmatter parses a YAML frontmatter block from markdown content.
// Returns the parsed Frontmatter, the body (content after frontmatter),
// and any parse error.
//
// Supports:
//   - Blocks between ---\n and \n---\n (standard Jekyll frontmatter)
//   - keys: description, tags, created, updated
//   - tags: inline (tags: go, cli) or multiline (tags:\n  - go\n  - cli)
//   - [[wikilinks]] in body (when parseLinks is true)
//
// No external YAML dependency — minimal parser for pi-memory-md frontmatter.
func ParseFrontmatter(content string, parseLinks bool) (Frontmatter, string, error) {
	var fm Frontmatter
	if len(content) < 4 || !strings.HasPrefix(content, "---\n") {
		return fm, content, nil // no frontmatter, full content is body
	}

	// Find closing ---
	// Start searching after the first "---\n" (4 chars)
	rest := content[4:]
	end := strings.Index(rest, "\n---\n")
	if end < 0 {
		// Try with no trailing newline: "...---\n" but content ends with "---"
		if trimmedRest := strings.TrimSuffix(rest, "\n"); strings.HasSuffix(trimmedRest, "---") {
			for i := len(trimmedRest) - 3; i >= 0; i-- {
				// Find "---" at end of a line
				if i == 0 || trimmedRest[i-1] == '\n' {
					end = i
					break
				}
			}
		}
	}
	if end < 0 {
		return fm, content, errors.New("frontmatter not closed: missing \\n---\\n delimiter")
	}

	block := content[4 : 4+end]
	// Calculate body end position. The delimiter is either "\n---\n" (5 chars) or just "---" (3 chars).
	bodyStart := 4 + end + 5
	if bodyStart > len(content) {
		bodyStart = 4 + end + 3
	}
	var body string
	if bodyStart < len(content) {
		body = content[bodyStart:]
	} else {
		body = ""
	}

	fm.Tags = nil
	multiLineTags := false

	lines := strings.Split(block, "\n")
	for i, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Handle multiline tag list after "tags:" key
		if multiLineTags {
			if strings.HasPrefix(line, "- ") {
				fm.Tags = append(fm.Tags, strings.TrimSpace(line[2:]))
				continue
			}
			// Next key starts → stop multiline mode
			if strings.Contains(line, ":") {
				multiLineTags = false
			} else {
				continue
			}
		}

		if strings.HasPrefix(line, "tags:") {
			rest := strings.TrimSpace(line[5:])
			if rest != "" {
				// inline tags: tags: go, cli or tags: [go, cli]
				fm.Tags = parseInlineTags(rest)
			} else {
				// multiline tags — look at next lines
				multiLineTags = true
				// Parse the next lines as tag list
				for j := i + 1; j < len(lines); j++ {
					nextLine := strings.TrimSpace(lines[j])
					if strings.HasPrefix(nextLine, "- ") {
						fm.Tags = append(fm.Tags, strings.TrimSpace(nextLine[2:]))
					} else if strings.Contains(nextLine, ":") || nextLine == "" {
						break
					}
				}
				multiLineTags = false
			}
			continue
		}

		if idx := strings.Index(line, ":"); idx > 0 {
			key := strings.TrimSpace(line[:idx])
			val := strings.Trim(strings.TrimSpace(line[idx+1:]), "'\"")
			switch key {
			case "description":
				fm.Description = val
			case "created":
				fm.Created = val
			case "updated":
				fm.Updated = val
			}
		}
	}

	if parseLinks {
		for _, m := range wikilinkRe.FindAllStringSubmatch(body, -1) {
			fm.Links = append(fm.Links, m[1])
		}
	}

	if fm.Tags == nil {
		fm.Tags = []string{}
	}

	return fm, body, nil
}

// parseInlineTags parses a simple tag list from a YAML inline value.
// Supports: "go, cli" and "[go, cli]" and "go cli"
func parseInlineTags(s string) []string {
	s = strings.TrimSpace(s)
	s = strings.Trim(s, "[]")

	if s == "" {
		return nil
	}

	// Try comma-separated first
	if strings.Contains(s, ",") {
		parts := strings.Split(s, ",")
		result := make([]string, 0, len(parts))
		for _, p := range parts {
			t := strings.TrimSpace(p)
			if t != "" {
				result = append(result, t)
			}
		}
		return result
	}

	// Space-separated
	return strings.Fields(s)
}

// FirstHeading returns the first markdown heading from body text.
func FirstHeading(body string) string {
	for _, line := range strings.Split(body, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "# ") {
			return strings.TrimSpace(trimmed[2:])
		}
		if strings.HasPrefix(trimmed, "## ") {
			return strings.TrimSpace(trimmed[3:])
		}
	}
	return ""
}

// FirstParagraph returns the first non-empty paragraph from body text.
func FirstParagraph(body string) string {
	var para strings.Builder
	inPara := false
	for _, line := range strings.Split(body, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			if inPara {
				break
			}
			continue
		}
		if strings.HasPrefix(trimmed, "#") {
			continue // skip headings
		}
		if !inPara {
			inPara = true
		}
		if para.Len() > 0 {
			para.WriteString(" ")
		}
		para.WriteString(trimmed)
		if para.Len() > 500 {
			break
		}
	}
	return strings.TrimSpace(para.String())
}
