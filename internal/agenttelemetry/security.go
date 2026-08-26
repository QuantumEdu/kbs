package agenttelemetry

import (
	"encoding/json"
	"fmt"
)

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
func (sp *SecurityPipeline) Process(e *Event) error {
	if e.RedactionPolicy == "hash-args" {
		if err := sp.hashProtectedPayload(e); err != nil {
			return err
		}
	}
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
	return nil
}

func (sp *SecurityPipeline) hashProtectedPayload(e *Event) error {
	var payload map[string]json.RawMessage
	if err := json.Unmarshal(e.Payload, &payload); err != nil {
		return fmt.Errorf("parse protected payload: %w", err)
	}
	for _, key := range []string{"command", "args", "arguments"} {
		if raw, ok := payload[key]; ok {
			payload["args_hash"] = json.RawMessage(fmt.Sprintf("%q", sp.hasher.Hash([]string{string(raw)})))
			delete(payload, key)
		}
	}
	delete(payload, "raw_line")
	transformed, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal protected payload: %w", err)
	}
	e.Payload = transformed
	return nil
}

// SaltFingerprint returns the first 8 hex chars of SHA-256(salt), or empty
// string when the pipeline has no hasher.
func (sp *SecurityPipeline) SaltFingerprint() string {
	if sp.hasher != nil {
		return sp.hasher.SaltFingerprint()
	}
	return ""
}

// RedactionPatterns returns the list of active regex pattern strings, or nil
// when the pipeline has no redactor.
func (sp *SecurityPipeline) RedactionPatterns() []string {
	if sp.redactor != nil {
		return sp.redactor.Patterns()
	}
	return nil
}
