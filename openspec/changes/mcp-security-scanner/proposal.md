# Proposal: Security Audit Engine & Import Gate for SkillVault

## Intent

As AI agent workflows proliferate, prompts, skills, workflows, and distributed skill packs (`.svpack`) have become attack surfaces for **indirect prompt injection**, **credential exfiltration**, and **destructive command execution**.

Inspired by [`traceforce/mcp-xray`](https://github.com/traceforce/mcp-xray), this change introduces a native, local-first **Security Audit Engine** into SkillVault (`kbs`) along with CLI audit tools (`skillvault audit`) and a pre-import security gate (`skillvault import --strict-audit`).

## User Stories

1. **As an AI Engineer / Developer**, I want to run `skillvault audit` against my vault or specific prompts/skills so I can detect prompt injection vulnerabilities, hardcoded secrets, and dangerous shell execution patterns before publishing or using them.
2. **As an Agent Operator**, I want `skillvault import --strict-audit` to inspect incoming `.svpack` packages and reject packages containing critical prompt injections or plaintext credentials.
3. **As a CI/CD Pipeline**, I want `skillvault audit` to return standard exit codes and structured JSON output so that insecure prompt changes can be caught and blocked during pull request checks.

## Scope

### Included in this Change (Phases 1 & 2):
* **Phase 1: Security Audit Engine (`internal/security/audit.go`)**:
  * Rules for Indirect Prompt Injection, Jailbreak signatures, System override markers.
  * Rules for API keys, tokens, and high-entropy secret leakage.
  * Rules for dangerous command patterns (`rm -rf /`, `curl | sh`, reverse shells).
  * Entry, File, and Pack audit routines.
* **Phase 2: CLI Command & Import Gating**:
  * `skillvault audit` command (support for full vault audit, single file audit, pack audit).
  * `skillvault import --strict-audit` flag to prevent importing tainted packs.
  * Formatted terminal table output with color/severity indicators and JSON output format (`--format json`).
