package github

import (
	"net/http"
	"reflect"
	"strings"
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

func TestListOpenDependabotPRs(t *testing.T) {
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
			transport := &fakeTransport{fixture: tt.fixture}

			got, err := newTestClient(t, nil, transport).ListOpenDependabotPRs()
			if err != nil {
				t.Fatalf("ListOpenDependabotPRs() error: %v", err)
			}

			assertRequestsMatchGolden(t, transport, "dependabot_prs")
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ListOpenDependabotPRs() =\n%+v\nwant\n%+v", got, tt.want)
			}
		})
	}
}

func TestListOpenDependabotPRsPaginates(t *testing.T) {
	transport := &fakeTransport{fixtures: []string{"pull_requests_page_1.json", "pull_requests_page_2.json"}}
	got, err := newTestClient(t, nil, transport).ListOpenDependabotPRs()
	if err != nil {
		t.Fatalf("ListOpenDependabotPRs() error: %v", err)
	}
	if len(got) != 1 || got[0].Number != 101 {
		t.Fatalf("ListOpenDependabotPRs() = %+v, want Dependabot PR #101 from page 2", got)
	}
	if len(transport.requests) != 2 {
		t.Fatalf("request count = %d, want 2", len(transport.requests))
	}
	if got := transport.requests[0].Variables["after"]; got != nil {
		t.Errorf("first request after = %v, want nil", got)
	}
	if got := transport.requests[1].Variables["after"]; got != "cursor-100" {
		t.Errorf("second request after = %v, want cursor-100", got)
	}
	assertRequestsMatchGolden(t, transport, "dependabot_prs_pagination")
}

func TestListOpenDependabotPRsReturnsErrorFromLaterPage(t *testing.T) {
	transport := &fakeTransport{
		fixtures: []string{"pull_requests_page_1.json", "pull_requests_page_2.json"},
		statuses: []int{http.StatusOK, http.StatusBadGateway},
	}
	got, err := newTestClient(t, nil, transport).ListOpenDependabotPRs()
	if err == nil || !strings.HasPrefix(err.Error(), "failed to query pull requests: ") {
		t.Fatalf("ListOpenDependabotPRs() = %v, %v; want wrapped later-page error", got, err)
	}
	if len(transport.requests) != 2 {
		t.Fatalf("request count = %d, want 2", len(transport.requests))
	}
}

func TestOpenDependabotPR(t *testing.T) {
	tests := []struct {
		name    string
		fixture string
		wantErr string
	}{
		{name: "returns one open Dependabot pull request with its full body", fixture: "pull_request_by_number.json"},
		{name: "returns not found for a missing pull request", fixture: "pull_request_by_number_null.json", wantErr: "pull request #42 not found"},
		{name: "rejects a closed pull request", fixture: "pull_request_by_number_closed.json", wantErr: "pull request #42 is not open"},
		{name: "rejects a pull request outside Dependabot scope", fixture: "pull_request_by_number_out_of_scope.json", wantErr: "pull request #42 is not a Dependabot pull request"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			transport := &fakeTransport{fixture: tt.fixture}
			got, err := newTestClient(t, nil, transport).OpenDependabotPR(42)
			assertErr(t, err, tt.wantErr)
			assertRequestsMatchGolden(t, transport, "dependabot_pr_by_number")
			if tt.wantErr != "" {
				return
			}
			want := PullRequestDetails{
				PullRequest: dependabot.PullRequest{
					Number: 42, Title: "Bump example from 1.0 to 1.1", Author: "dependabot[bot]",
					HeadRefName: "dependabot/npm_and_yarn/example-1.1", Checks: dependabot.CheckPending,
					Mergeable: dependabot.Mergeable,
				},
				Labels: []string{"dependencies", "status: Do_Not_Merge"},
				Body:   "Bumps example from 1.0 to 1.1.\n\nRelease notes: full body.",
			}
			if !reflect.DeepEqual(got, want) {
				t.Errorf("OpenDependabotPR(42) =\n%+v\nwant\n%+v", got, want)
			}
		})
	}
}

func TestOpenDependabotPRPaginatesChecks(t *testing.T) {
	tests := []struct {
		name   string
		status int
	}{
		{name: "includes a failing check on the next page", status: http.StatusOK},
		{name: "returns an error for an incomplete rollup", status: http.StatusBadGateway},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			transport := &fakeTransport{
				fixtures: []string{"pull_request_checks_first_page.json", "pull_request_checks_next_page.json"},
				statuses: []int{http.StatusOK, tt.status},
			}
			got, err := newTestClient(t, nil, transport).OpenDependabotPR(42)
			if tt.status != http.StatusOK {
				if err == nil || !strings.Contains(err.Error(), "failed to query checks for PR #42") || !reflect.DeepEqual(got, PullRequestDetails{}) {
					t.Fatalf("OpenDependabotPR(42) = %+v, %v; want no details and a check error", got, err)
				}
				return
			}
			if err != nil || got.PullRequest.Checks != dependabot.CheckFailing {
				t.Fatalf("OpenDependabotPR(42) = %+v, %v; want failing checks", got, err)
			}
			assertRequestsMatchGolden(t, transport, "dependabot_checks_pagination")
		})
	}
}

func TestOpenDependabotPRPaginatesLabels(t *testing.T) {
	transport := &fakeTransport{fixtures: []string{
		"pull_request_labels_page_1.json",
		"pull_request_labels_page_2.json",
	}}
	got, err := newTestClient(t, nil, transport).OpenDependabotPR(42)
	if err != nil {
		t.Fatalf("OpenDependabotPR(42) error: %v", err)
	}
	want := []string{"dependencies", "team review", "hold"}
	if !reflect.DeepEqual(got.Labels, want) {
		t.Errorf("OpenDependabotPR(42).Labels = %v, want %v", got.Labels, want)
	}
	if len(transport.requests) != 2 {
		t.Fatalf("request count = %d, want 2", len(transport.requests))
	}
	if got := transport.requests[1].Variables["after"]; got != "labels-next" {
		t.Errorf("second request after = %v, want labels-next", got)
	}
	assertRequestsMatchGolden(t, transport, "dependabot_labels_pagination")
}

func TestOpenDependabotPRReturnsErrorForInvalidLabelCursor(t *testing.T) {
	transport := &fakeTransport{fixture: "pull_request_labels_missing_cursor.json"}
	got, err := newTestClient(t, nil, transport).OpenDependabotPR(42)
	if err == nil || !strings.Contains(err.Error(), "failed to query labels for PR #42: invalid pagination data") {
		t.Fatalf("OpenDependabotPR(42) = %+v, %v; want invalid labels pagination error", got, err)
	}
	if len(transport.requests) != 1 {
		t.Errorf("request count = %d, want 1", len(transport.requests))
	}
}

func TestOpenDependabotPRReturnsErrorForRepeatedLabelCursor(t *testing.T) {
	transport := &fakeTransport{fixtures: []string{
		"pull_request_labels_page_1.json",
		"pull_request_labels_repeated_cursor.json",
	}}
	got, err := newTestClient(t, nil, transport).OpenDependabotPR(42)
	if err == nil || !strings.Contains(err.Error(), "failed to query labels for PR #42: invalid pagination data") {
		t.Fatalf("OpenDependabotPR(42) = %+v, %v; want invalid labels pagination error", got, err)
	}
	if len(transport.requests) != 2 {
		t.Errorf("request count = %d, want 2", len(transport.requests))
	}
}

func TestOpenDependabotPRReturnsErrorWhenLaterLabelPageIsUnavailable(t *testing.T) {
	transport := &fakeTransport{fixtures: []string{
		"pull_request_labels_page_1.json",
		"pull_request_labels_null.json",
	}}
	got, err := newTestClient(t, nil, transport).OpenDependabotPR(42)
	if err == nil || !strings.Contains(err.Error(), "failed to query labels for PR #42: labels unavailable") {
		t.Fatalf("OpenDependabotPR(42) = %+v, %v; want unavailable labels error", got, err)
	}
}
