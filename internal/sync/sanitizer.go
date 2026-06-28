package sync

import (
	"io"
	"regexp"
)

// sanitizeValueRE matches credential patterns with their values.
// Three alternations: JSON-style "key":"value", bare key="value", and bare key=value.
var sanitizeValueRE = regexp.MustCompile(
	`(?i)("\w*(?:key|secret|token)\w*"\s*:\s*")([^"]*)(")` +
		`|(\b\w*(?:key|secret|token)\w*\s*[:=]\s*")([^"]*)(")` +
		`|(\b\w*(?:key|secret|token)\w*\s*[:=]\s*)(\S+)`,
)

// SanitizingWriter wraps an io.Writer and replaces credential values
// with ***REDACTED*** in the output stream.
type SanitizingWriter struct {
	w io.Writer
}

// NewSanitizingWriter creates a writer that masks credential patterns.
func NewSanitizingWriter(w io.Writer) *SanitizingWriter {
	return &SanitizingWriter{w: w}
}

// Write sanitizes the data and writes it to the underlying writer.
// Returns the original byte count to avoid confusing io.Writer callers.
func (s *SanitizingWriter) Write(p []byte) (int, error) {
	sanitized := sanitizeValueRE.ReplaceAllFunc(p, func(match []byte) []byte {
		sub := sanitizeValueRE.FindSubmatch(match)
		switch {
		case len(sub[1]) > 0: // JSON-style: "key":"value"
			return []byte(string(sub[1]) + "***REDACTED***" + string(sub[3]))
		case len(sub[4]) > 0: // bare key="value"
			return []byte(string(sub[4]) + "***REDACTED***" + string(sub[6]))
		case len(sub[7]) > 0: // bare key=value
			return []byte(string(sub[7]) + "***REDACTED***")
		}
		return match
	})
	_, err := s.w.Write(sanitized)
	return len(p), err
}
