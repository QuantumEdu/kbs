package version

import (
	"regexp"
	"testing"
)

func TestNumberIsSemanticVersion(t *testing.T) {
	semverRe := regexp.MustCompile(`^\d+\.\d+\.\d+$`)
	if !semverRe.MatchString(Number) {
		t.Errorf("Number = %q, want MAJOR.MINOR.PATCH format", Number)
	}
}

func TestDisplayPrefixesNumber(t *testing.T) {
	want := "v" + Number
	if got := Display(); got != want {
		t.Errorf("Display() = %q, want %q", got, want)
	}
}
