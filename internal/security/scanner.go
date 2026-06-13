package security

import "regexp"

type Match struct {
	Pattern string
	Type    string
	Start   int
	End     int
}

type ScanResult struct {
	HasSecret bool
	Matches   []Match
}

type secretPattern struct {
	re   *regexp.Regexp
	name string
}

var patterns = []secretPattern{
	{regexp.MustCompile(`sk-[A-Za-z0-9_-]{20,}`), "openai_api_key"},
	{regexp.MustCompile(`-----BEGIN (RSA |EC |OPENSSH |)?PRIVATE KEY-----`), "private_key"},
	{regexp.MustCompile(`ghp_[A-Za-z0-9_]{20,}`), "github_token"},
	{regexp.MustCompile(`xox[baprs]-[A-Za-z0-9-]{20,}`), "slack_token"},
}

type SecretScanner struct{}

func New() *SecretScanner {
	return &SecretScanner{}
}

func (s *SecretScanner) Scan(content string) (ScanResult, error) {
	var matches []Match

	for _, p := range patterns {
		locs := p.re.FindAllStringIndex(content, -1)
		for _, loc := range locs {
			matches = append(matches, Match{
				Pattern: content[loc[0]:loc[1]],
				Type:    p.name,
				Start:   loc[0],
				End:     loc[1],
			})
		}
	}

	return ScanResult{
		HasSecret: len(matches) > 0,
		Matches:   matches,
	}, nil
}

func (s *SecretScanner) Redact(content string) (string, []Match) {
	result, _ := s.Scan(content)
	if !result.HasSecret {
		return content, nil
	}

	redacted := content
	var redactedMatches []Match

	for _, p := range patterns {
		redacted = p.re.ReplaceAllStringFunc(redacted, func(match string) string {
			redactedMatches = append(redactedMatches, Match{
				Pattern: match,
				Type:    p.name,
			})
			return "[REDACTED]"
		})
	}

	return redacted, redactedMatches
}
