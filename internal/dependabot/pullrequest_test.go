package dependabot

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func loadConclusions(t *testing.T, name string) []string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "checks.json"))
	if err != nil {
		t.Fatalf("reading fixture: %v", err)
	}
	var sets map[string][]string
	if err := json.Unmarshal(data, &sets); err != nil {
		t.Fatalf("decoding fixture: %v", err)
	}
	conclusions, ok := sets[name]
	if !ok {
		t.Fatalf("no check set %q in fixture", name)
	}
	return conclusions
}

func TestCheckStateOf(t *testing.T) {
	tests := []struct {
		fixture    string
		want       CheckState
		wantPassed bool
	}{
		{"none", CheckNone, true},
		{"all_passing", CheckPassing, true},
		{"passing_with_skipped", CheckPassing, true},
		{"skipped_only", CheckPassing, true},
		{"failing_among_pending", CheckFailing, false},
		{"error_among_passing", CheckFailing, false},
		{"pending_only", CheckPending, false},
		{"pending_among_passing", CheckPending, false},
		{"unknown_conclusion", CheckPending, false},
	}
	for _, tt := range tests {
		t.Run(tt.fixture, func(t *testing.T) {
			got := CheckStateOf(loadConclusions(t, tt.fixture))
			if got != tt.want {
				t.Errorf("CheckStateOf(%s) = %v, want %v", tt.fixture, got, tt.want)
			}
			if got.Passed() != tt.wantPassed {
				t.Errorf("CheckStateOf(%s).Passed() = %v, want %v", tt.fixture, got.Passed(), tt.wantPassed)
			}
		})
	}
}

func TestCheckStateOfNil(t *testing.T) {
	if got := CheckStateOf(nil); got != CheckNone {
		t.Errorf("CheckStateOf(nil) = %v, want %v", got, CheckNone)
	}
}

func TestEligible(t *testing.T) {
	tests := []struct {
		name      string
		checks    CheckState
		mergeable Mergeability
		want      bool
	}{
		{"no checks and mergeable", CheckNone, Mergeable, true},
		{"passing and mergeable", CheckPassing, Mergeable, true},
		{"passing but conflicting", CheckPassing, Conflicting, false},
		{"passing but unknown", CheckPassing, Unknown, false},
		{"passing but empty mergeable", CheckPassing, "", false},
		{"failing but mergeable", CheckFailing, Mergeable, false},
		{"pending but mergeable", CheckPending, Mergeable, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pr := PullRequest{Checks: tt.checks, Mergeable: tt.mergeable}
			if got := pr.Eligible(); got != tt.want {
				t.Errorf("Eligible(%v, %q) = %v, want %v", tt.checks, tt.mergeable, got, tt.want)
			}
		})
	}
}

func TestIsDependabot(t *testing.T) {
	tests := []struct {
		name        string
		login       string
		headRefName string
		want        bool
	}{
		{"bot login", "dependabot[bot]", "main", true},
		{"app login", "app/dependabot", "main", true},
		{"plain login", "dependabot", "main", true},
		{"dependabot branch, other author", "someone", "dependabot/go_modules/foo-1.2.3", true},
		{"dependabot branch, no author", "", "dependabot/npm_and_yarn/bar-4.5.6", true},
		{"human PR", "someone", "feat/thing", false},
		{"lookalike login", "dependabot-preview", "feat/thing", false},
		{"branch prefix without slash", "someone", "dependabot-fix", false},
		{"dependabot in the middle of the branch", "someone", "fix/dependabot/foo", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pr := PullRequest{Author: tt.login, HeadRefName: tt.headRefName}
			if got := pr.IsDependabot(); got != tt.want {
				t.Errorf("IsDependabot(%q, %q) = %v, want %v", tt.login, tt.headRefName, got, tt.want)
			}
		})
	}
}

func TestFilterByNumbers(t *testing.T) {
	prs := []PullRequest{{Number: 1}, {Number: 2}, {Number: 3}}

	tests := []struct {
		name    string
		numbers []int
		want    []int
	}{
		{"subset keeps PR order", []int{3, 1}, []int{1, 3}},
		{"all", []int{1, 2, 3}, []int{1, 2, 3}},
		{"unknown numbers ignored", []int{2, 42}, []int{2}},
		{"duplicates", []int{2, 2}, []int{2}},
		{"no match", []int{42}, nil},
		{"no numbers", nil, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got []int
			for _, pr := range FilterByNumbers(prs, tt.numbers) {
				got = append(got, pr.Number)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("FilterByNumbers(%v) = %v, want %v", tt.numbers, got, tt.want)
			}
		})
	}
}
