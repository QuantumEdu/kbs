package line

import "testing"

func TestParseOutcome(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		stdout  string
		want    Outcome
		wantErr bool
	}{
		{
			name:   "ok object",
			stdout: `{"status":"ok","summary":"refined the issue"}`,
			want:   Outcome{Status: OutcomeOK, Summary: "refined the issue"},
		},
		{
			name:   "needs human with question",
			stdout: "noise\n{\"status\":\"needs_human\",\"question\":\"Is the API public?\"}\n",
			want:   Outcome{Status: OutcomeNeedsHuman, Question: "Is the API public?"},
		},
		{
			name:   "blocked",
			stdout: `{"status":"blocked","summary":"gh auth missing"}`,
			want:   Outcome{Status: OutcomeBlocked, Summary: "gh auth missing"},
		},
		{
			name:    "missing json",
			stdout:  "just prose",
			wantErr: true,
		},
		{
			name:    "unknown status",
			stdout:  `{"status":"maybe"}`,
			wantErr: true,
		},
		{
			name:    "needs human without question",
			stdout:  `{"status":"needs_human"}`,
			wantErr: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := ParseOutcome(tc.stdout)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got %+v", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseOutcome: %v", err)
			}
			if got != tc.want {
				t.Fatalf("got %+v want %+v", got, tc.want)
			}
		})
	}
}
