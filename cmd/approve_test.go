package cmd

import (
	"reflect"
	"testing"
)

func TestHasPassingChecks(t *testing.T) {
	tests := []struct {
		fixture string
		want    bool
	}{
		{"none", true},
		{"all_passing", true},
		{"passing_with_skipped", true},
		{"skipped_only", true},
		{"failing_among_pending", false},
		{"error_among_passing", false},
		{"pending_only", false},
		{"pending_among_passing", false},
		{"unknown_conclusion", false},
	}
	for _, tt := range tests {
		t.Run(tt.fixture, func(t *testing.T) {
			if got := hasPassingChecks(loadChecks(t, tt.fixture)); got != tt.want {
				t.Errorf("hasPassingChecks(%s) = %v, want %v", tt.fixture, got, tt.want)
			}
		})
	}
}

func TestIsEligible(t *testing.T) {
	tests := []struct {
		name      string
		fixture   string
		mergeable string
		want      bool
	}{
		{"no checks and mergeable", "none", "MERGEABLE", true},
		{"passing and mergeable", "all_passing", "MERGEABLE", true},
		{"skipped and mergeable", "passing_with_skipped", "MERGEABLE", true},
		{"passing but conflicting", "all_passing", "CONFLICTING", false},
		{"passing but unknown", "all_passing", "UNKNOWN", false},
		{"passing but empty mergeable", "all_passing", "", false},
		{"failing but mergeable", "failing_among_pending", "MERGEABLE", false},
		{"pending but mergeable", "pending_only", "MERGEABLE", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isEligible(loadChecks(t, tt.fixture), tt.mergeable); got != tt.want {
				t.Errorf("isEligible(%s, %q) = %v, want %v", tt.fixture, tt.mergeable, got, tt.want)
			}
		})
	}
}

func TestIsDependabotPR(t *testing.T) {
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
			if got := isDependabotPR(tt.login, tt.headRefName); got != tt.want {
				t.Errorf("isDependabotPR(%q, %q) = %v, want %v", tt.login, tt.headRefName, got, tt.want)
			}
		})
	}
}

func TestFilterByNumbers(t *testing.T) {
	prs := []pullRequest{{Number: 1}, {Number: 2}, {Number: 3}}

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
			for _, pr := range filterByNumbers(prs, tt.numbers) {
				got = append(got, pr.Number)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("filterByNumbers(%v) = %v, want %v", tt.numbers, got, tt.want)
			}
		})
	}
}

func TestApproveCommand(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want [][]string
	}{
		{
			name: "approves every eligible pull request",
			args: []string{"approve"},
			want: [][]string{
				{"pr", "review", "1", "--approve"},
				{"pr", "review", "4", "--approve"},
			},
		},
		{
			name: "approves only the requested pull requests",
			args: []string{"approve", "4", "5"},
			want: [][]string{
				{"pr", "review", "4", "--approve"},
			},
		},
		{
			name: "skips pull requests that are not eligible",
			args: []string{"approve", "3"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fake := newFakeGitHub(t, map[string]string{"DependabotPRsForApproval": "pull_requests.json"})

			if err := runCommand(t, tt.args...); err != nil {
				t.Fatalf("approve error: %v", err)
			}

			fake.assertRequestsMatchGolden("approve")
			fake.assertExecs(tt.want)
		})
	}
}
