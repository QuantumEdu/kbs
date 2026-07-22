//go:build otel

// Package agenttelemetry provides OpenTelemetry integration when built
// with the "otel" build tag. Without this tag, no OTel SDK code is
// compiled or linked.
//
// Build: go build -tags otel ./...
//
// This file is a Phase 2 plumbing stub. In Phase 2, it will wire
// OTel trace/span creation and metrics export for agent telemetry events.
package agenttelemetry

// otelEnabled is true when this file is compiled (the "otel" tag is active).
const otelEnabled = true
