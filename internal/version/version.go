// Package version defines the application's semantic version.
//
// Versioning scheme (semver, MAJOR.MINOR.PATCH):
//   - MAJOR: generation/redesign; bump only on breaking changes (v4 when warranted).
//   - MINOR: unbounded; backward-compatible features and improvements. Bump per merged feature PR.
//   - PATCH: unbounded; backward-compatible fixes and minor changes. Bump per merged fix PR.
package version

// Number is the semantic version MAJOR.MINOR.PATCH.
const Number = "3.0.0"

// Display returns the user-facing form with the leading v.
func Display() string { return "v" + Number }
