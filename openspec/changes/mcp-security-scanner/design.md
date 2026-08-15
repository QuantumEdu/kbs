# Design: Security Audit Engine & CLI Import Gate

## Technical Approach

```mermaid
flowchart LR
    Target[Input: File / Pack / Vault Entry] --> Auditor[internal/security/Auditor]
    Auditor --> RuleEngine[Rule Engine]
    
    subgraph RuleSet[Rule Set]
        Inj[Prompt Injection Matchers]
        Sec[Secret & Entropy Matchers]
        Cmd[Command Hazard Matchers]
    end
    
    RuleEngine --> Inj
    RuleEngine --> Sec
    RuleEngine --> Cmd
    
    RuleEngine --> Report[AuditReport]
    Report --> CLI[CLI Output / Exit Code Gate]
    Report --> ImportGate[Import Service Gate]
```

### Components

1. **`internal/security/audit.go`**:
   - `Auditor`: Main engine with regex compilation at init.
   - `Finding`: Data model with `RuleID`, `Category`, `Severity`, `Description`, `MatchSnippet`, `LineNumber`, `Suggestion`.
   - `AuditReport`: Container with summary counts and list of findings.
   - Methods:
     - `AuditContent(target string, content string) AuditReport`
     - `AuditFile(path string) (AuditReport, error)`
     - `AuditPack(data []byte) (AuditReport, error)`
     - `AuditVaultEntries(entries []domain.Entry) (AuditReport, error)`

2. **`internal/app/audit.go`**:
   - `AuditService`: Coordinates database queries with the security auditor to scan active vault entries.

3. **`internal/cli/flags.go`**:
   - `ParseAuditFlags(args []string) (AuditFlags, error)`
   - Update `ParseImportFlags` to include `StrictAudit bool` (`--strict-audit`).

4. **`cmd/skillvault/main.go`**:
   - Case `"audit"`: Executes vault, pack, or file audit and formats output.
   - Case `"import"`: If `--strict-audit` is set, calls `Auditor.AuditPack()` or `Auditor.AuditFile()` before persisting to SQLite.
