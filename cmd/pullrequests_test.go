package cmd

import (
	"reflect"
	"testing"

	"github.com/knplabs/gh-dependabot/internal/dependabot"
)

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

func TestFetchDependabotPullRequests(t *testing.T) {
	tests := []struct {
		name    string
		fixture string
		want    []dependabot.PullRequest
	}{
		{
			name:    "maps checks and keeps dependabot pull requests only",
			fixture: "pull_requests.json",
			want: []dependabot.PullRequest{
				{
					Number:      1,
					Title:       "Bump charm.land/bubbles/v2 from 2.2.0 to 2.2.1",
					Author:      "dependabot",
					HeadRefName: "dependabot/go_modules/charm.land/bubbles/v2-2.2.1",
					Checks:      dependabot.CheckPassing,
					Mergeable:   dependabot.Mergeable,
				},
				{
					Number:      3,
					Title:       "Bump actions/checkout from 4 to 5",
					Author:      "octocat",
					HeadRefName: "dependabot/github_actions/actions/checkout-5",
					Checks:      dependabot.CheckPending,
					Mergeable:   dependabot.Conflicting,
				},
				{
					Number:      4,
					Title:       "Bump github.com/spf13/cobra from 1.10.1 to 1.10.2",
					Author:      "dependabot",
					HeadRefName: "dependabot/go_modules/github.com/spf13/cobra-1.10.2",
					Checks:      dependabot.CheckNone,
					Mergeable:   dependabot.Mergeable,
				},
				{
					Number:      5,
					Title:       "Bump golang.org/x/sys from 0.45.0 to 0.46.0",
					Author:      "dependabot",
					HeadRefName: "dependabot/go_modules/golang.org/x/sys-0.46.0",
					Checks:      dependabot.CheckFailing,
					Mergeable:   dependabot.Mergeable,
				},
			},
		},
		{
			name:    "null status check rollup means no checks",
			fixture: "pull_request_rollup_null.json",
			want: []dependabot.PullRequest{
				{
					Number:      10,
					Title:       "Bump foo from 1.0 to 1.1",
					Author:      "dependabot",
					HeadRefName: "dependabot/npm_and_yarn/foo-1.1",
					Checks:      dependabot.CheckNone,
					Mergeable:   dependabot.Mergeable,
				},
			},
		},
		{
			name:    "no commits means no checks",
			fixture: "pull_request_no_commits.json",
			want: []dependabot.PullRequest{
				{
					Number:      11,
					Title:       "Bump bar from 2.0 to 3.0",
					Author:      "dependabot",
					HeadRefName: "dependabot/npm_and_yarn/bar-3.0",
					Checks:      dependabot.CheckNone,
					Mergeable:   dependabot.Unknown,
				},
			},
		},
		{
			name:    "status contexts only are normalised",
			fixture: "pull_request_status_context_only.json",
			want: []dependabot.PullRequest{
				{
					Number:      12,
					Title:       "Bump baz from 0.1 to 0.2",
					Author:      "dependabot",
					HeadRefName: "dependabot/pip/baz-0.2",
					Checks:      dependabot.CheckPending,
					Mergeable:   dependabot.Mergeable,
				},
				{
					Number:      13,
					Title:       "Bump qux from 5.0 to 5.1",
					Author:      "dependabot",
					HeadRefName: "dependabot/pip/qux-5.1",
					Checks:      dependabot.CheckPassing,
					Mergeable:   dependabot.Mergeable,
				},
			},
		},
		{
			name:    "non dependabot pull requests are filtered out",
			fixture: "pull_request_non_dependabot.json",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fake := newFakeGitHub(t, map[string]string{"DependabotPRs": tt.fixture})

			got, err := fetchDependabotPullRequests()
			if err != nil {
				t.Fatalf("fetchDependabotPullRequests() error: %v", err)
			}

			fake.assertRequestsMatchGolden("list")
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("fetchDependabotPullRequests() =\n%+v\nwant\n%+v", got, tt.want)
			}
		})
	}
}
