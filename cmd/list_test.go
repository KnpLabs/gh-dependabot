package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"unicode/utf8"
)

func loadChecks(t *testing.T, name string) []statusCheck {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "checks.json"))
	if err != nil {
		t.Fatalf("reading fixture: %v", err)
	}
	var sets map[string][]statusCheck
	if err := json.Unmarshal(data, &sets); err != nil {
		t.Fatalf("decoding fixture: %v", err)
	}
	checks, ok := sets[name]
	if !ok {
		t.Fatalf("no check set %q in fixture", name)
	}
	return checks
}

func TestMapStateToConclusion(t *testing.T) {
	tests := []struct {
		state string
		want  string
	}{
		{"SUCCESS", "SUCCESS"},
		{"PENDING", "PENDING"},
		{"EXPECTED", "PENDING"},
		{"FAILURE", "FAILURE"},
		{"ERROR", "FAILURE"},
		{"", ""},
		{"SOMETHING_NEW", "SOMETHING_NEW"},
	}
	for _, tt := range tests {
		t.Run(tt.state, func(t *testing.T) {
			if got := mapStateToConclusion(tt.state); got != tt.want {
				t.Errorf("mapStateToConclusion(%q) = %q, want %q", tt.state, got, tt.want)
			}
		})
	}
}

func TestChecksStatus(t *testing.T) {
	tests := []struct {
		fixture string
		want    string
	}{
		{"none", "none"},
		{"all_passing", "✓ passing"},
		{"passing_with_skipped", "✓ passing"},
		{"skipped_only", "✓ passing"},
		{"failing_among_pending", "✗ failing"},
		{"error_among_passing", "✗ failing"},
		{"pending_only", "● pending"},
		{"pending_among_passing", "● pending"},
		{"unknown_conclusion", "● pending"},
	}
	for _, tt := range tests {
		t.Run(tt.fixture, func(t *testing.T) {
			if got := checksStatus(loadChecks(t, tt.fixture)); got != tt.want {
				t.Errorf("checksStatus(%s) = %q, want %q", tt.fixture, got, tt.want)
			}
		})
	}
}

func TestChecksStatusNil(t *testing.T) {
	if got := checksStatus(nil); got != "none" {
		t.Errorf("checksStatus(nil) = %q, want %q", got, "none")
	}
}

func TestTruncateTitle(t *testing.T) {
	tests := []struct {
		name  string
		title string
		max   int
		want  string
	}{
		{"shorter than max", "Bump foo", 20, "Bump foo"},
		{"exact length", "Bump foo", 8, "Bump foo"},
		{"ascii truncated", "Bump foo from 1.0 to 2.0", 10, "Bump foo…"},
		{"trailing space trimmed", "Bump foo bar", 6, "Bump…"},
		{"utf-8 exact length in runes", "café", 4, "café"},
		{"utf-8 truncated", "Mise à jour de café", 8, "Mise à…"},
		{"emoji not split", "🚀 Bump lodash", 3, "🚀…"},
		{"multi-rune emoji kept whole", "⬆️ Bump lodash from 4.17.20 to 4.17.21", 12, "⬆️ Bump lod…"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncateTitle(tt.title, tt.max)
			if got != tt.want {
				t.Errorf("truncateTitle(%q, %d) = %q, want %q", tt.title, tt.max, got, tt.want)
			}
			if !utf8.ValidString(got) {
				t.Errorf("truncateTitle(%q, %d) returned invalid UTF-8: %q", tt.title, tt.max, got)
			}
			if n := utf8.RuneCountInString(got); n > tt.max {
				t.Errorf("truncateTitle(%q, %d) is %d runes long, want at most %d", tt.title, tt.max, n, tt.max)
			}
		})
	}
}

func TestFetchDependabotPullRequests(t *testing.T) {
	fake := newFakeGitHub(t, map[string]string{"DependabotPRs": "pull_requests.json"})

	got, err := fetchDependabotPullRequests()
	if err != nil {
		t.Fatalf("fetchDependabotPullRequests() error: %v", err)
	}

	fake.assertRequestsMatchGolden("list")

	want := []listPullRequest{
		{
			Number:            1,
			Title:             "Bump charm.land/bubbles/v2 from 2.2.0 to 2.2.1",
			Author:            author{Login: "dependabot"},
			StatusCheckRollup: []statusCheck{{Conclusion: "SUCCESS"}, {Conclusion: "SKIPPED"}},
			Mergeable:         "MERGEABLE",
		},
		{
			Number:            3,
			Title:             "Bump actions/checkout from 4 to 5",
			Author:            author{Login: "octocat"},
			StatusCheckRollup: []statusCheck{{Conclusion: "PENDING"}},
			Mergeable:         "CONFLICTING",
		},
		{
			Number:    4,
			Title:     "Bump github.com/spf13/cobra from 1.10.1 to 1.10.2",
			Author:    author{Login: "dependabot"},
			Mergeable: "MERGEABLE",
		},
		{
			Number:            5,
			Title:             "Bump golang.org/x/sys from 0.45.0 to 0.46.0",
			Author:            author{Login: "dependabot"},
			StatusCheckRollup: []statusCheck{{Conclusion: "SUCCESS"}, {Conclusion: "FAILURE"}},
			Mergeable:         "MERGEABLE",
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("fetchDependabotPullRequests() =\n%+v\nwant\n%+v", got, want)
	}
}
