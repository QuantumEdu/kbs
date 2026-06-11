package vars

import (
	"reflect"
	"testing"
)

func TestDetectVars(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected []string
	}{
		{
			name:     "single variable",
			content:  "Hello {{name}}",
			expected: []string{"name"},
		},
		{
			name:     "multiple variables",
			content:  "{{greeting}} {{name}}, welcome to {{project}}",
			expected: []string{"greeting", "name", "project"},
		},
		{
			name:     "no variables",
			content:  "Hello world",
			expected: nil,
		},
		{
			name:     "empty content",
			content:  "",
			expected: nil,
		},
		{
			name:     "variables with special chars",
			content:  "{{project_name}} {{stack-version}}",
			expected: []string{"project_name", "stack-version"},
		},
		{
			name:     "case sensitive detection",
			content:  "{{Project}} and {{project}}",
			expected: []string{"Project", "project"},
		},
		{
			name:     "duplicate variables",
			content:  "{{name}} {{name}} {{name}}",
			expected: []string{"name"},
		},
		{
			name:     "partial braces without closing",
			content:  "{{unclosed",
			expected: nil,
		},
		{
			name:     "nested braces detect innermost",
			content:  "{{outer {{inner}} }}",
			expected: []string{"inner"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Detect(tt.content)
			if !reflect.DeepEqual(got, tt.expected) {
				t.Errorf("Detect(%q) = %v, want %v", tt.content, got, tt.expected)
			}
		})
	}
}

func TestResolveVars(t *testing.T) {
	tests := []struct {
		name         string
		content      string
		providedVars map[string]string
		globals      map[string]string
		wantContent  string
		wantMissing  []string
	}{
		{
			name:         "replaces all variables",
			content:      "Hello {{name}}, your role is {{role}}",
			providedVars: map[string]string{"name": "World", "role": "admin"},
			wantContent:  "Hello World, your role is admin",
			wantMissing:  nil,
		},
		{
			name:         "missing variables left visible",
			content:      "{{name}} {{role}}",
			providedVars: map[string]string{"name": "A"},
			wantContent:  "A {{role}}",
			wantMissing:  []string{"role"},
		},
		{
			name:         "no variables in content",
			content:      "Hello world",
			providedVars: map[string]string{"name": "X"},
			wantContent:  "Hello world",
			wantMissing:  nil,
		},
		{
			name:         "case sensitive resolution",
			content:      "{{Project}} {{project}}",
			providedVars: map[string]string{"project": "vitacare"},
			wantContent:  "{{Project}} vitacare",
			wantMissing:  []string{"Project"},
		},
		{
			name:         "global date injection",
			content:      "Today is {{date}}",
			globals:      map[string]string{"date": "2026-06-11"},
			wantContent:  "Today is 2026-06-11",
			wantMissing:  nil,
		},
		{
			name:         "global project injection",
			content:      "Working on {{project}}",
			globals:      map[string]string{"project": "vitacare"},
			wantContent:  "Working on vitacare",
			wantMissing:  nil,
		},
		{
			name:         "provided vars override globals",
			content:      "{{date}}",
			providedVars: map[string]string{"date": "custom-date"},
			globals:      map[string]string{"date": "2026-06-11"},
			wantContent:  "custom-date",
			wantMissing:  nil,
		},
		{
			name:         "multiple missing vars",
			content:      "{{a}} {{b}} {{c}}",
			providedVars: map[string]string{"a": "1"},
			wantContent:  "1 {{b}} {{c}}",
			wantMissing:  []string{"b", "c"},
		},
		{
			name:         "empty content",
			content:      "",
			providedVars: map[string]string{"x": "y"},
			wantContent:  "",
			wantMissing:  nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotContent, gotMissing := Resolve(tt.content, tt.providedVars, tt.globals)
			if gotContent != tt.wantContent {
				t.Errorf("Resolve() content = %q, want %q", gotContent, tt.wantContent)
			}
			if !reflect.DeepEqual(gotMissing, tt.wantMissing) {
				t.Errorf("Resolve() missing = %v, want %v", gotMissing, tt.wantMissing)
			}
		})
	}
}
