package github

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/cli/go-gh/v2/pkg/api"
	"github.com/cli/go-gh/v2/pkg/repository"
	"github.com/knplabs/gh-dependabot/internal/dependabot"
)

var errExit = errors.New("exit status 1")

type execResult struct {
	stdout, stderr string
	err            error
}

type fakeExec struct {
	calls     [][]string
	responses map[string]execResult
}

func (f *fakeExec) exec(args ...string) (stdout, stderr bytes.Buffer, err error) {
	f.calls = append(f.calls, args)
	r := f.responses[args[1]]
	stdout.WriteString(r.stdout)
	stderr.WriteString(r.stderr)
	return stdout, stderr, r.err
}

var update = flag.Bool("update", false, "rewrite golden files in testdata/golden")

type graphQLRequest struct {
	Query     string         `json:"query"`
	Variables map[string]any `json:"variables"`
}

type fakeTransport struct {
	t        *testing.T
	fixture  string
	status   int
	requests []graphQLRequest
}

func (f *fakeTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	var body graphQLRequest
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		f.t.Fatalf("decoding GraphQL request: %v", err)
	}
	f.requests = append(f.requests, body)

	data, err := os.ReadFile(filepath.Join("testdata", f.fixture))
	if err != nil {
		f.t.Fatalf("reading fixture: %v", err)
	}
	status := f.status
	if status == 0 {
		status = http.StatusOK
	}
	return &http.Response{
		StatusCode: status,
		Header:     http.Header{"Content-Type": {"application/json"}},
		Body:       io.NopCloser(bytes.NewReader(data)),
		Request:    req,
	}, nil
}

var fakeRepo = repository.Repository{Host: "github.com", Owner: "knplabs", Name: "fake-repo"}

func newTestClient(t *testing.T, exec *fakeExec, transport *fakeTransport) Client {
	t.Helper()
	if exec == nil {
		exec = &fakeExec{}
	}
	if transport == nil {
		transport = &fakeTransport{t: t}
	}
	transport.t = t
	graphql, err := api.NewGraphQLClient(api.ClientOptions{
		Host:      fakeRepo.Host,
		AuthToken: "fake-token",
		Transport: transport,
	})
	if err != nil {
		t.Fatalf("creating GraphQL client: %v", err)
	}
	return New(exec.exec, graphql, fakeRepo)
}

func assertCalls(t *testing.T, got, want [][]string) {
	t.Helper()
	if len(got) == 0 && len(want) == 0 {
		return
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("gh calls mismatch\n got: %q\nwant: %q", got, want)
	}
}

func assertErr(t *testing.T, err error, want string) {
	t.Helper()
	if want == "" {
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		return
	}
	if err == nil || err.Error() != want {
		t.Fatalf("error = %v, want %q", err, want)
	}
}

func TestApprove(t *testing.T) {
	tests := []struct {
		name    string
		result  execResult
		wantErr string
	}{
		{name: "approves the pull request"},
		{
			name:    "stderr becomes the error message",
			result:  execResult{stderr: "  GraphQL: Review cannot be requested from pull request author.\n", err: errExit},
			wantErr: "GraphQL: Review cannot be requested from pull request author.",
		},
		{
			name:    "falls back to the exec error without stderr",
			result:  execResult{err: errExit},
			wantErr: "exit status 1",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exec := &fakeExec{responses: map[string]execResult{"review": tt.result}}

			err := newTestClient(t, exec, nil).Approve(42)

			assertErr(t, err, tt.wantErr)
			assertCalls(t, exec.calls, [][]string{{"pr", "review", "42", "--approve"}})
		})
	}
}

func TestMerge(t *testing.T) {
	tests := []struct {
		name         string
		method       dependabot.MergeMethod
		deleteBranch bool
		responses    map[string]execResult
		wantErr      string
		wantConflict bool
		want         [][]string
	}{
		{
			name:         "squashes and deletes the branch",
			method:       dependabot.MergeMethodSquash,
			deleteBranch: true,
			want:         [][]string{{"pr", "merge", "7", "--squash", "--delete-branch"}},
		},
		{
			name:   "keeps the branch",
			method: dependabot.MergeMethodMerge,
			want:   [][]string{{"pr", "merge", "7", "--merge"}},
		},
		{
			name:         "merge commit that cannot be created is a conflict",
			method:       dependabot.MergeMethodRebase,
			deleteBranch: true,
			responses: map[string]execResult{
				"merge": {stderr: "Pull request is not mergeable: the merge commit cannot be cleanly created.", err: errExit},
			},
			wantErr:      "merge conflict: Pull request is not mergeable: the merge commit cannot be cleanly created.",
			wantConflict: true,
			want:         [][]string{{"pr", "merge", "7", "--rebase", "--delete-branch"}},
		},
		{
			name:   "conflicting pull request is a conflict",
			method: dependabot.MergeMethodSquash,
			responses: map[string]execResult{
				"merge": {stderr: "GraphQL: Base branch was modified.", err: errExit},
				"view":  {stdout: "CONFLICTING\n"},
			},
			wantErr:      "merge conflict: GraphQL: Base branch was modified.",
			wantConflict: true,
			want: [][]string{
				{"pr", "merge", "7", "--squash"},
				{"pr", "view", "7", "--json", "mergeable", "--jq", ".mergeable"},
			},
		},
		{
			name:   "unrelated failure is not a conflict",
			method: dependabot.MergeMethodSquash,
			responses: map[string]execResult{
				"merge": {stderr: "GraphQL: Required status check is expected.", err: errExit},
				"view":  {stdout: "MERGEABLE\n"},
			},
			wantErr: "GraphQL: Required status check is expected.",
			want: [][]string{
				{"pr", "merge", "7", "--squash"},
				{"pr", "view", "7", "--json", "mergeable", "--jq", ".mergeable"},
			},
		},
		{
			name:   "failing to read mergeability is not a conflict",
			method: dependabot.MergeMethodSquash,
			responses: map[string]execResult{
				"merge": {stderr: "GraphQL: Base branch was modified.", err: errExit},
				"view":  {stdout: "CONFLICTING\n", err: errExit},
			},
			wantErr: "GraphQL: Base branch was modified.",
			want: [][]string{
				{"pr", "merge", "7", "--squash"},
				{"pr", "view", "7", "--json", "mergeable", "--jq", ".mergeable"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exec := &fakeExec{responses: tt.responses}

			err := newTestClient(t, exec, nil).Merge(7, tt.method, tt.deleteBranch)

			assertErr(t, err, tt.wantErr)
			if got := errors.Is(err, ErrConflict); got != tt.wantConflict {
				t.Errorf("errors.Is(err, ErrConflict) = %v, want %v", got, tt.wantConflict)
			}
			assertCalls(t, exec.calls, tt.want)
		})
	}
}

func TestRequestRebase(t *testing.T) {
	tests := []struct {
		name    string
		result  execResult
		wantErr string
	}{
		{name: "comments on the pull request"},
		{
			name:    "stderr becomes the error message",
			result:  execResult{stderr: "GraphQL: Resource not accessible by integration\n", err: errExit},
			wantErr: "GraphQL: Resource not accessible by integration",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exec := &fakeExec{responses: map[string]execResult{"comment": tt.result}}

			err := newTestClient(t, exec, nil).RequestRebase(3)

			assertErr(t, err, tt.wantErr)
			assertCalls(t, exec.calls, [][]string{{"pr", "comment", "3", "--body", "@dependabot rebase"}})
		})
	}
}

func TestDiff(t *testing.T) {
	tests := []struct {
		name     string
		result   execResult
		wantDiff string
		wantErr  string
	}{
		{
			name:     "returns the diff",
			result:   execResult{stdout: "diff --git a/go.mod b/go.mod\n"},
			wantDiff: "diff --git a/go.mod b/go.mod\n",
		},
		{
			name:    "stderr becomes the error message",
			result:  execResult{stdout: "partial", stderr: "could not find pull request\n", err: errExit},
			wantErr: "could not find pull request",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exec := &fakeExec{responses: map[string]execResult{"diff": tt.result}}

			diff, err := newTestClient(t, exec, nil).Diff(5)

			assertErr(t, err, tt.wantErr)
			if diff != tt.wantDiff {
				t.Errorf("Diff() = %q, want %q", diff, tt.wantDiff)
			}
			assertCalls(t, exec.calls, [][]string{{"pr", "diff", "5"}})
		})
	}
}

func TestAllowedMergeMethods(t *testing.T) {
	transport := &fakeTransport{fixture: "merge_methods_all.json"}

	got, err := newTestClient(t, nil, transport).AllowedMergeMethods()
	if err != nil {
		t.Fatalf("AllowedMergeMethods() error: %v", err)
	}

	want := dependabot.AllowedMergeMethods{Merge: true, Squash: true, Rebase: true}
	if got != want {
		t.Errorf("AllowedMergeMethods() = %+v, want %+v", got, want)
	}
	assertRequestsMatchGolden(t, transport, "repo_merge_methods")
}

func TestGraphQLErrors(t *testing.T) {
	tests := []struct {
		name    string
		call    func(Client) error
		wantErr string
	}{
		{
			name: "pull requests",
			call: func(c Client) error {
				_, err := c.ListOpenDependabotPRs()
				return err
			},
			wantErr: "failed to query pull requests: ",
		},
		{
			name: "merge methods",
			call: func(c Client) error {
				_, err := c.AllowedMergeMethods()
				return err
			},
			wantErr: "failed to query repository merge settings: ",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			transport := &fakeTransport{fixture: "merge_methods_all.json", status: http.StatusBadGateway}

			err := tt.call(newTestClient(t, nil, transport))
			if err == nil || !strings.HasPrefix(err.Error(), tt.wantErr) {
				t.Fatalf("error = %v, want prefix %q", err, tt.wantErr)
			}
		})
	}
}

func assertRequestsMatchGolden(t *testing.T, transport *fakeTransport, name string) {
	t.Helper()

	var b strings.Builder
	for i, r := range transport.requests {
		if i > 0 {
			b.WriteString("\n---\n\n")
		}
		variables, err := json.Marshal(r.Variables)
		if err != nil {
			t.Fatalf("encoding variables: %v", err)
		}
		fmt.Fprintf(&b, "%s\n\nvariables: %s\n", r.Query, variables)
	}
	got := b.String()

	path := filepath.Join("testdata", "golden", name+".graphql")
	if *update {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}

	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading golden file (run `go test ./internal/github -update` to create it): %v", err)
	}
	if got != string(want) {
		t.Errorf("GraphQL requests differ from %s (run `go test ./internal/github -update` if the change is intended)\n--- got ---\n%s\n--- want ---\n%s", path, got, want)
	}
}
