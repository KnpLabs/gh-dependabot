package cmd

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/knplabs/gh-dependabot/internal/dependabot"
	"github.com/knplabs/gh-dependabot/internal/github"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

var openPullRequests = []dependabot.PullRequest{
	{Number: 1, Title: "Bump bubbles", Checks: dependabot.CheckPassing, Mergeable: dependabot.Mergeable},
	{Number: 3, Title: "Bump checkout", Checks: dependabot.CheckPending, Mergeable: dependabot.Conflicting},
	{Number: 4, Title: "Bump cobra", Checks: dependabot.CheckNone, Mergeable: dependabot.Mergeable},
	{Number: 5, Title: "Bump x/sys", Checks: dependabot.CheckFailing, Mergeable: dependabot.Mergeable},
}

type fakeClient struct {
	prs        []dependabot.PullRequest
	listErr    error
	allowed    dependabot.AllowedMergeMethods
	approveErr map[int]error
	mergeErr   map[int]error
	rebaseErr  map[int]error
	calls      []string
}

func newFakeClient(t *testing.T) *fakeClient {
	t.Helper()
	f := &fakeClient{
		prs:     openPullRequests,
		allowed: dependabot.AllowedMergeMethods{Merge: true, Squash: true, Rebase: true},
	}

	origClient := newClient
	t.Cleanup(func() { newClient = origClient })
	newClient = func() (github.Client, error) { return f, nil }

	return f
}

func (f *fakeClient) ListOpenDependabotPRs() ([]dependabot.PullRequest, error) {
	f.calls = append(f.calls, "ListOpenDependabotPRs")
	return f.prs, f.listErr
}

func (f *fakeClient) AllowedMergeMethods() (dependabot.AllowedMergeMethods, error) {
	f.calls = append(f.calls, "AllowedMergeMethods")
	return f.allowed, nil
}

func (f *fakeClient) Approve(number int) error {
	f.calls = append(f.calls, fmt.Sprintf("Approve %d", number))
	return f.approveErr[number]
}

func (f *fakeClient) Merge(number int, method dependabot.MergeMethod, deleteBranch bool) error {
	f.calls = append(f.calls, fmt.Sprintf("Merge %d %s deleteBranch=%t", number, method, deleteBranch))
	return f.mergeErr[number]
}

func (f *fakeClient) RequestRebase(number int) error {
	f.calls = append(f.calls, fmt.Sprintf("RequestRebase %d", number))
	return f.rebaseErr[number]
}

func (f *fakeClient) Diff(number int) (string, error) {
	f.calls = append(f.calls, fmt.Sprintf("Diff %d", number))
	return fmt.Sprintf("diff of #%d", number), nil
}

func (f *fakeClient) assertCalls(t *testing.T, want []string) {
	t.Helper()
	if len(want) == 0 && len(f.calls) == 0 {
		return
	}
	if !reflect.DeepEqual(f.calls, want) {
		t.Errorf("client calls mismatch\n got: %q\nwant: %q", f.calls, want)
	}
}

var update = flag.Bool("update", false, "rewrite golden files in testdata/golden")

type commandResult struct {
	stdout string
	stderr string
	err    error
}

func runCommand(t *testing.T, args ...string) commandResult {
	t.Helper()
	var stdout, stderr bytes.Buffer
	rootCmd.SetArgs(args)
	rootCmd.SetOut(&stdout)
	rootCmd.SetErr(&stderr)
	t.Cleanup(func() {
		rootCmd.SetArgs(nil)
		rootCmd.SetOut(nil)
		rootCmd.SetErr(nil)
		resetFlags(rootCmd)
	})
	err := rootCmd.Execute()
	return commandResult{stdout: stdout.String(), stderr: stderr.String(), err: err}
}

func resetFlags(cmd *cobra.Command) {
	cmd.Flags().VisitAll(func(f *pflag.Flag) {
		_ = f.Value.Set(f.DefValue)
		f.Changed = false
	})
	for _, sub := range cmd.Commands() {
		resetFlags(sub)
	}
}

func assertOutput(t *testing.T, got commandResult, wantStdout, wantStderr string) {
	t.Helper()
	if got.err != nil {
		t.Fatalf("unexpected error: %v", got.err)
	}
	if got.stdout != wantStdout {
		t.Errorf("stdout mismatch\n got: %q\nwant: %q", got.stdout, wantStdout)
	}
	if got.stderr != wantStderr {
		t.Errorf("stderr mismatch\n got: %q\nwant: %q", got.stderr, wantStderr)
	}
}

func assertGolden(t *testing.T, name, got string) {
	t.Helper()
	path := filepath.Join("testdata", "golden", name)
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
		t.Fatalf("reading golden file (run `go test ./cmd -update` to create it): %v", err)
	}
	if got != string(want) {
		t.Errorf("output differs from %s (run `go test ./cmd -update` if the change is intended)\n--- got ---\n%s\n--- want ---\n%s", path, got, want)
	}
}

func assertCommandError(t *testing.T, got commandResult, want string) {
	t.Helper()
	if got.err == nil || got.err.Error() != want {
		t.Fatalf("error = %v, want %q", got.err, want)
	}
}
