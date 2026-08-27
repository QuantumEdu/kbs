package main

import "testing"

func TestParseUsageLineAcceptsOnlyBoundedProviderPayload(t *testing.T) {
	payload, ok := parseUsageLine(`{"provider":"opencode","model":"gpt-5","effort":"high","usage":{"input":12,"output":3,"cache_read":2,"cache_write":1,"reasoning":4},"sample_id":"req-1","interaction_id":"turn-1"}`)
	if !ok || payload["provider"] != "opencode" || payload["model"] != "gpt-5" || payload["schema_version"] != 1 {
		t.Fatalf("payload = %#v, ok=%t", payload, ok)
	}
}

func TestParseUsageLineRejectsUnsupportedWrapperOutput(t *testing.T) {
	if payload, ok := parseUsageLine("tokens: 12 input, 3 output"); ok || payload["coverage"] != "unknown" {
		t.Fatalf("unsupported wrapper payload = %#v, ok=%t", payload, ok)
	}
}
