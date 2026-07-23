package agenttelemetry

import (
	"fmt"
	"regexp"
	"strings"
)

// builtinPatterns are the 4 required redaction regexes.
var builtinPatterns = []string{
	`sk-[a-zA-Z0-9\-]{32,}`,       // OpenAI keys
	`Bearer\s+[a-zA-Z0-9._\-]+`,  // Bearer tokens
	`Authorization:\s*\S+(?:\s+\S+)?`, // Auth headers (scheme + optional value)
	`--api-key\s+\S+`,            // CLI API key flags
}

// Redactor applies regex-based redaction to strings.
type Redactor struct {
	patterns []*regexp.Regexp
	raw      []string
}

// NewRedactor compiles built-in and custom redaction patterns. Invalid custom
// patterns produce an error but the redactor still works with built-in patterns.
func NewRedactor(customPatterns []string) (*Redactor, []error) {
	allPatterns := append([]string{}, builtinPatterns...)
	allPatterns = append(allPatterns, customPatterns...)

	var compiled []*regexp.Regexp
	var errors []error
	var raw []string

	for _, p := range allPatterns {
		re, err := regexp.Compile(p)
		if err != nil {
			errors = append(errors, fmt.Errorf("compile pattern %q: %w", p, err))
			continue
		}
		compiled = append(compiled, re)
		raw = append(raw, p)
	}

	return &Redactor{patterns: compiled, raw: raw}, errors
}

// Redact applies all compiled redaction patterns to s.
func (r *Redactor) Redact(s string) string {
	result := s
	for _, re := range r.patterns {
		result = re.ReplaceAllStringFunc(result, func(match string) string {
			switch {
			case strings.HasPrefix(match, "sk-"):
				return "sk-***REDACTED***"
			case strings.HasPrefix(match, "Bearer "):
				return "Bearer ***REDACTED***"
			case strings.HasPrefix(match, "Authorization:"):
				return "Authorization: ***REDACTED***"
			case strings.HasPrefix(match, "--api-key"):
				return "--api-key ***REDACTED***"
			default:
				return "***REDACTED***"
			}
		})
	}
	return result
}

// Patterns returns the list of active regex pattern strings.
func (r *Redactor) Patterns() []string {
	return append([]string{}, r.raw...)
}
