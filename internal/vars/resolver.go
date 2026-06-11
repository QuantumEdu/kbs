package vars

import "strings"

// Resolve replaces {{key}} placeholders in content with values from the provided
// variables map. Global variables (like {{date}}, {{project}}) are used as fallback
// when a key is not in the provided vars. Missing variables are left as {{key}}
// and collected in the returned missing list.
//
// Returns the resolved content and a list of variable names that were not found.
func Resolve(content string, providedVars map[string]string, globals map[string]string) (string, []string) {
	if content == "" {
		return "", nil
	}

	var missing []string
	seen := make(map[string]bool)

	result := varPattern.ReplaceAllStringFunc(content, func(match string) string {
		key := match[2 : len(match)-2]

		// Check provided vars first
		if val, ok := providedVars[key]; ok {
			return val
		}

		// Check globals
		if val, ok := globals[key]; ok {
			return val
		}

		// Missing — collect once
		if !seen[key] {
			seen[key] = true
			missing = append(missing, key)
		}

		// Leave the placeholder visible
		return match
	})

	// Order: preserve first-occurrence order
	sorted := make([]string, 0, len(missing))
	allVars := Detect(content)
	for _, v := range allVars {
		if seen[v] {
			sorted = append(sorted, v)
		}
	}
	if len(sorted) == 0 {
		sorted = nil
	}

	return result, sorted
}

// PrepareGlobals creates a globals map with standard values.
func PrepareGlobals(projectID *string) map[string]string {
	g := make(map[string]string)
	g["date"] = "" // Will be filled by caller with current date
	if projectID != nil {
		g["project"] = *projectID
	}
	return g
}

// FindAll repeatedly calls strings.ReplaceAll for each key in the map.
// Deprecated: use Resolve instead.
func FindAll(template string, vars map[string]string) string {
	for k, v := range vars {
		placeholder := "{{" + k + "}}"
		template = strings.ReplaceAll(template, placeholder, v)
	}
	return template
}
