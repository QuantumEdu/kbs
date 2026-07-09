# Delta for Entry Purpose Taxonomy

## MODIFIED Requirements

### Requirement: Purpose Value Set (REQ-PUR-01)

The system SHALL support six purpose values: `WORK`, `KNOWLEDGE`, `LEARNING`, `RELATIONSHIP`, `STATE`, `OBSERVABILITY`.
(Previously: five — OBSERVABILITY was deferred)

#### Scenario: OBSERVABILITY accepted
- GIVEN OBSERVABILITY is a valid purpose
- WHEN `save_entry` called with `purpose: "OBSERVABILITY"`
- THEN entry persisted successfully

#### Scenario: Invalid purpose still rejected
- GIVEN valid purposes now include OBSERVABILITY
- WHEN `save_entry` called with `purpose: "INVALID_VALUE"`
- THEN rejected with validation error
