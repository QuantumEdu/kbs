package agenttelemetry

import "encoding/json"

// SecurityPipeline orchestrates redaction, entropy scanning, and hashing on
// incoming telemetry events. It is wired into the Collector's ingest path.
type SecurityPipeline struct {
	hasher   *ArgHasher
	redactor *Redactor
	scanner  *EntropyScanner
}

// NewSecurityPipeline creates a security pipeline backed by a salt file at
// saltPath. Custom regex redaction patterns are compiled alongside the
// built-in patterns. Returns an error if the salt cannot be loaded/created
// or has incorrect permissions.
func NewSecurityPipeline(saltPath string, customPatterns []string) (*SecurityPipeline, error) {
	sm, err := NewSaltManager(saltPath)
	if err != nil {
		return nil, err
	}

	if err := ValidateSaltPerms(saltPath); err != nil {
		return nil, err
	}

	redactor, _ := NewRedactor(customPatterns) // custom pattern compile errors are non-fatal

	return &SecurityPipeline{
		hasher:   NewArgHasher(sm.Salt()),
		redactor: redactor,
		scanner:  NewEntropyScanner(),
	}, nil
}

// Process applies the security pipeline to an event in-place:
//  1. Redacts the event's Payload field (JSON string).
//  2. Scans the redacted payload for high-entropy tokens.
//  3. Sets RedactionPolicy to "scanned-warning" if a high-entropy token is
//     found, unless the policy is already "hash-args".
func (sp *SecurityPipeline) Process(e *Event) {
	// 1. Redact payload.
	redacted := sp.redactor.Redact(string(e.Payload))
	e.Payload = json.RawMessage(redacted)

	// 2. Entropy scan.
	if sp.scanner.Scan(redacted) {
		// Only upgrade to scanned-warning if not already explicitly hash-args.
		if e.RedactionPolicy == "" || e.RedactionPolicy == "none" {
			e.RedactionPolicy = "scanned-warning"
		}
	}
}
