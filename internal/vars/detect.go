package vars

import "regexp"

// varPattern matches {{key}} where key contains word chars, hyphens, underscores.
var varPattern = regexp.MustCompile(`\{\{[a-zA-Z0-9_\-]+\}\}`)

// Detect finds all unique variable names in the content.
// Returns nil if no variables are found.
func Detect(content string) []string {
	matches := varPattern.FindAllString(content, -1)
	if len(matches) == 0 {
		return nil
	}

	seen := make(map[string]bool)
	result := make([]string, 0, len(matches))
	for _, m := range matches {
		// Strip {{ and }}
		name := m[2 : len(m)-2]
		if !seen[name] {
			seen[name] = true
			result = append(result, name)
		}
	}
	return result
}
