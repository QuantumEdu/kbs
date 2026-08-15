package agenttelemetry

import (
	"regexp"
	"sync"
	"time"
)

// InjectionSignal is emitted when a prompt injection or command hazard is intercepted in telemetry.
type InjectionSignal struct {
	Signal       string    `json:"signal"` // "injection.detected"
	RuleID       string    `json:"rule_id"`
	Category     string    `json:"category"`
	Severity     string    `json:"severity"`
	Description  string    `json:"description"`
	MatchSnippet string    `json:"match_snippet"`
	EventType    string    `json:"event_type"`
	DetectedAt   time.Time `json:"detected_at"`
}

type telemetrySecurityRule struct {
	id          string
	category    string
	severity    string
	description string
	re          *regexp.Regexp
}

// InjectionDetector monitors live telemetry events for prompt injections and destructive command hazards.
type InjectionDetector struct {
	mu    sync.Mutex
	rules []telemetrySecurityRule
}

// NewInjectionDetector creates an InjectionDetector initialized with prompt injection and command hazard rules.
func NewInjectionDetector() *InjectionDetector {
	return &InjectionDetector{
		rules: []telemetrySecurityRule{
			{
				id:          "INJ-001",
				category:    "prompt_injection",
				severity:    "critical",
				description: "Instruction override or system prompt hijacking phrase detected",
				re:          regexp.MustCompile(`(?i)\b(ignore\s+(all\s+)?(previous|prior)\s+instructions|disregard\s+(all\s+)?(previous|prior)\s+instructions|system\s+override)\b`),
			},
			{
				id:          "INJ-002",
				category:    "prompt_injection",
				severity:    "high",
				description: "Jailbreak mode or safety filter bypass marker detected",
				re:          regexp.MustCompile(`(?i)\b(you\s+are\s+now\s+in\s+developer\s+mode|dan\s+mode\s+enabled|jailbreak\s+(mode|enabled|active)|bypass\s+(all\s+)?safety\s+filters)\b`),
			},
			{
				id:          "INJ-003",
				category:    "prompt_injection",
				severity:    "high",
				description: "System tag spoofing or delimiter injection detected",
				re:          regexp.MustCompile(`(?i)(<\s*/?\s*(system|admin|override)\s*>|\\u003c\s*/?\s*(system|admin|override)\s*\\u003e|\[\s*(system|system_prompt)\s*\])`),
			},
			{
				id:          "CMD-001",
				category:    "dangerous_command",
				severity:    "critical",
				description: "Destructive filesystem deletion or disk wiping command",
				re:          regexp.MustCompile(`(?i)\b(rm\s+-(?:rf|fr|r|f)\s+([/~]|\*|\S+)|mkfs\.[a-z0-9]+|dd\s+if=\S+\s+of=/dev/\S*)`),
			},
			{
				id:          "CMD-002",
				category:    "dangerous_command",
				severity:    "high",
				description: "Unsafe remote script execution via pipe to shell",
				re:          regexp.MustCompile(`(?i)\b(curl|wget)\s+[^\n|;]+\|\s*(bash|sh|zsh)\b`),
			},
			{
				id:          "CMD-003",
				category:    "dangerous_command",
				severity:    "critical",
				description: "Potential reverse shell or raw network execution command",
				re:          regexp.MustCompile(`(?i)\b(nc\s+-[a-z0-9]*e\s+/bin/(ba)?sh|/dev/tcp/[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+)`),
			},
		},
	}
}

// Check scans an event payload for prompt injection or command hazard signatures.
// Returns an InjectionSignal if a violation is detected, or nil otherwise.
func (d *InjectionDetector) Check(e Event) *InjectionSignal {
	d.mu.Lock()
	defer d.mu.Unlock()

	payloadStr := string(e.Payload)
	if len(payloadStr) == 0 {
		return nil
	}

	for _, rule := range d.rules {
		match := rule.re.FindString(payloadStr)
		if match != "" {
			snippet := match
			if len(snippet) > 60 {
				snippet = snippet[:57] + "..."
			}
			return &InjectionSignal{
				Signal:       "injection.detected",
				RuleID:       rule.id,
				Category:     rule.category,
				Severity:     rule.severity,
				Description:  rule.description,
				MatchSnippet: snippet,
				EventType:    e.EventType,
				DetectedAt:   time.Now(),
			}
		}
	}

	return nil
}
