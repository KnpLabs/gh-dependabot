package cmd

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"github.com/knplabs/gh-dependabot/internal/dependabot"
	"github.com/knplabs/gh-dependabot/internal/github"
)

type tuiDriver struct {
	t      *testing.T
	model  interactiveModel
	client *fakeClient
	quit   bool
}

func newTUIDriver(t *testing.T, prs []dependabot.PullRequest, configure func(*interactiveModel)) *tuiDriver {
	t.Helper()
	client := &fakeClient{prs: prs}
	d := &tuiDriver{t: t, model: newInteractiveModel(client), client: client}
	if configure != nil {
		configure(&d.model)
	}
	d.settle(d.run(d.model.Init())...)
	return d
}

func (d *tuiDriver) send(msg tea.Msg) []tea.Msg {
	d.t.Helper()
	next, cmd := d.model.Update(msg)
	model, ok := next.(interactiveModel)
	if !ok {
		d.t.Fatalf("Update returned %T, want interactiveModel", next)
	}
	d.model = model
	return d.run(cmd)
}

func (d *tuiDriver) settle(msgs ...tea.Msg) {
	d.t.Helper()
	for len(msgs) > 0 {
		msg := msgs[0]
		msgs = append(msgs[1:], d.send(msg)...)
	}
}

func (d *tuiDriver) press(key string) []tea.Msg {
	d.t.Helper()
	return d.send(keyPress(key))
}

func (d *tuiDriver) run(cmd tea.Cmd) []tea.Msg {
	if cmd == nil {
		return nil
	}
	switch msg := cmd().(type) {
	case tea.BatchMsg:
		var msgs []tea.Msg
		for _, c := range msg {
			msgs = append(msgs, d.run(c)...)
		}
		return msgs
	case tea.QuitMsg:
		d.quit = true
		return nil
	case spinner.TickMsg:
		return nil
	default:
		return []tea.Msg{msg}
	}
}

func (d *tuiDriver) assertState(want tuiState) {
	d.t.Helper()
	got := tuiState{
		approved: d.model.approved,
		merged:   d.model.merged,
		rebased:  d.model.rebased,
		skipped:  d.model.skipped,
		failed:   failedNumbers(d.model),
		pending:  d.model.pending,
		quit:     d.quit,
	}
	if !reflect.DeepEqual(got, want) {
		d.t.Errorf("state mismatch\n got: %+v\nwant: %+v", got, want)
	}
}

type tuiState struct {
	approved []int
	merged   []int
	rebased  []int
	skipped  []int
	failed   []string
	pending  int
	quit     bool
}

func failedNumbers(m interactiveModel) []string {
	var failed []string
	for _, group := range []struct {
		kind     string
		failures []prFailure
	}{
		{"approve", m.approveFailed},
		{"merge", m.mergeFailed},
		{"rebase", m.rebaseFailed},
	} {
		for _, f := range group.failures {
			failed = append(failed, fmt.Sprintf("%s #%d: %v", group.kind, f.number, f.err))
		}
	}
	return failed
}

func keyPress(key string) tea.KeyPressMsg {
	switch key {
	case "enter":
		return tea.KeyPressMsg{Code: tea.KeyEnter}
	case "ctrl+c":
		return tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl}
	default:
		r := []rune(key)[0]
		return tea.KeyPressMsg{Code: r, Text: key}
	}
}

var reviewQueue = []dependabot.PullRequest{
	{Number: 1, Title: "Bump bubbles", Checks: dependabot.CheckPassing, Mergeable: dependabot.Mergeable},
	{Number: 5, Title: "Bump x/sys", Checks: dependabot.CheckFailing, Mergeable: dependabot.Mergeable},
}

func TestKeyPressStrings(t *testing.T) {
	for _, key := range []string{"y", "Y", "n", "r", "q", "enter", "ctrl+c"} {
		if got := keyPress(key).String(); got != key {
			t.Errorf("keyPress(%q).String() = %q", key, got)
		}
	}
}

func TestInteractiveEmptyListQuits(t *testing.T) {
	d := newTUIDriver(t, nil, nil)

	d.assertState(tuiState{quit: true})
	if !d.model.done {
		t.Error("model is not done")
	}
	d.client.assertCalls(t, []string{"ListOpenDependabotPRs"})
}

func TestInteractiveListErrorQuits(t *testing.T) {
	client := &fakeClient{listErr: errors.New("failed to query pull requests: boom")}
	d := &tuiDriver{t: t, model: newInteractiveModel(client), client: client}

	d.settle(d.run(d.model.Init())...)

	d.assertState(tuiState{quit: true})
	if got := d.model.View().Content; got != "Error: failed to query pull requests: boom\n" {
		t.Errorf("View() = %q", got)
	}
}

func TestInteractiveLoadsFirstDiff(t *testing.T) {
	d := newTUIDriver(t, reviewQueue, nil)

	if d.model.loadingDiff || d.model.diff != "diff of #1" {
		t.Errorf("loadingDiff = %v, diff = %q, want the diff of #1", d.model.loadingDiff, d.model.diff)
	}
	d.client.assertCalls(t, []string{"ListOpenDependabotPRs", "Diff 1"})
}

func TestInteractiveKeys(t *testing.T) {
	errConflict := fmt.Errorf("%w: base branch was modified", github.ErrConflict)

	tests := []struct {
		name      string
		merge     bool
		mergeErr  map[int]error
		rebaseErr map[int]error
		keys      []string
		want      tuiState
		wantCalls []string
	}{
		{
			name: "enter approves when checks pass and skips when they fail",
			keys: []string{"enter", "enter"},
			want: tuiState{approved: []int{1}, skipped: []int{5}, quit: true},
			wantCalls: []string{
				"ListOpenDependabotPRs", "Diff 1",
				"Approve 1", "Diff 5",
			},
		},
		{
			name: "y approves even when checks fail and n skips",
			keys: []string{"n", "y"},
			want: tuiState{approved: []int{5}, skipped: []int{1}, quit: true},
			wantCalls: []string{
				"ListOpenDependabotPRs", "Diff 1",
				"Diff 5",
				"Approve 5",
			},
		},
		{
			name:  "y with --merge chains a merge after the approval",
			merge: true,
			keys:  []string{"y", "n"},
			want:  tuiState{approved: []int{1}, merged: []int{1}, skipped: []int{5}, quit: true},
			wantCalls: []string{
				"ListOpenDependabotPRs", "Diff 1",
				"Approve 1", "Diff 5",
				"Merge 1 squash deleteBranch=true",
			},
		},
		{
			name:     "a merge conflict ends up in rebased",
			merge:    true,
			mergeErr: map[int]error{1: errConflict},
			keys:     []string{"y", "n"},
			want: tuiState{
				approved: []int{1},
				rebased:  []int{1},
				skipped:  []int{5},
				failed:   []string{"merge #1: merge conflict: base branch was modified"},
				quit:     true,
			},
			wantCalls: []string{
				"ListOpenDependabotPRs", "Diff 1",
				"Approve 1", "Diff 5",
				"Merge 1 squash deleteBranch=true", "RequestRebase 1",
			},
		},
		{
			name:      "a failed rebase after a conflict is reported",
			merge:     true,
			mergeErr:  map[int]error{1: errConflict},
			rebaseErr: map[int]error{1: errors.New("Resource not accessible by integration")},
			keys:      []string{"y", "n"},
			want: tuiState{
				approved: []int{1},
				skipped:  []int{5},
				failed: []string{
					"merge #1: merge conflict: base branch was modified",
					"rebase #1: Resource not accessible by integration",
				},
				quit: true,
			},
			wantCalls: []string{
				"ListOpenDependabotPRs", "Diff 1",
				"Approve 1", "Diff 5",
				"Merge 1 squash deleteBranch=true", "RequestRebase 1",
			},
		},
		{
			name:     "an unrelated merge failure does not request a rebase",
			merge:    true,
			mergeErr: map[int]error{1: errors.New("Required status check is expected.")},
			keys:     []string{"y", "n"},
			want: tuiState{
				approved: []int{1},
				skipped:  []int{5},
				failed:   []string{"merge #1: Required status check is expected."},
				quit:     true,
			},
			wantCalls: []string{
				"ListOpenDependabotPRs", "Diff 1",
				"Approve 1", "Diff 5",
				"Merge 1 squash deleteBranch=true",
			},
		},
		{
			name: "r requests a rebase and moves on",
			keys: []string{"r", "n"},
			want: tuiState{rebased: []int{1}, skipped: []int{5}, quit: true},
			wantCalls: []string{
				"ListOpenDependabotPRs", "Diff 1",
				"RequestRebase 1", "Diff 5",
			},
		},
		{
			name: "q quits right away when nothing is in flight",
			keys: []string{"n", "q"},
			want: tuiState{skipped: []int{1}, quit: true},
			wantCalls: []string{
				"ListOpenDependabotPRs", "Diff 1",
				"Diff 5",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := newTUIDriver(t, reviewQueue, func(m *interactiveModel) {
				m.mergeAfterApprove = tt.merge
				m.mergeMethod = dependabot.MergeMethodSquash
				m.deleteBranch = true
			})
			d.client.mergeErr = tt.mergeErr
			d.client.rebaseErr = tt.rebaseErr

			for _, key := range tt.keys {
				d.settle(d.press(key)...)
			}

			d.assertState(tt.want)
			d.client.assertCalls(t, tt.wantCalls)
		})
	}
}

func TestInteractiveQuitWaitsForInFlightRequests(t *testing.T) {
	for _, key := range []string{"q", "ctrl+c"} {
		t.Run(key, func(t *testing.T) {
			d := newTUIDriver(t, reviewQueue, nil)

			inFlight := d.press("y")
			d.assertState(tuiState{pending: 1})

			d.settle(d.press(key)...)
			d.assertState(tuiState{pending: 1})
			if got := d.model.View().Content; !strings.Contains(got, "Waiting for 1 in-flight request(s) to complete...") {
				t.Errorf("View() = %q, want the in-flight message", got)
			}

			d.settle(inFlight...)
			d.assertState(tuiState{approved: []int{1}, quit: true})

			var out strings.Builder
			d.model.printSummary(&out)
			if !strings.Contains(out.String(), "Unreviewed (1): #5\n") {
				t.Errorf("summary does not list #5 as unreviewed:\n%s", out.String())
			}
		})
	}
}

func TestInteractiveIgnoresStaleDiff(t *testing.T) {
	d := newTUIDriver(t, reviewQueue, nil)

	nextDiff := d.press("n")
	if d.model.index != 1 || !d.model.loadingDiff {
		t.Fatalf("index = %d, loadingDiff = %v, want the diff of #5 loading", d.model.index, d.model.loadingDiff)
	}

	d.settle(diffLoadedMsg{number: 1, diff: "stale diff of #1"})
	if !d.model.loadingDiff || d.model.diff != "" {
		t.Errorf("stale diff applied: loadingDiff = %v, diff = %q", d.model.loadingDiff, d.model.diff)
	}

	d.settle(nextDiff...)
	if d.model.loadingDiff || d.model.diff != "diff of #5" {
		t.Errorf("loadingDiff = %v, diff = %q, want the diff of #5", d.model.loadingDiff, d.model.diff)
	}
}

func TestInteractiveDiffError(t *testing.T) {
	d := newTUIDriver(t, reviewQueue, nil)

	d.settle(d.press("n")...)
	d.settle(diffLoadedMsg{number: 5, err: errors.New("could not find pull request")})

	if d.model.diff != "Failed to load diff: could not find pull request" {
		t.Errorf("diff = %q", d.model.diff)
	}
}

func TestDefaultApprove(t *testing.T) {
	tests := []struct {
		checks dependabot.CheckState
		want   bool
	}{
		{dependabot.CheckNone, true},
		{dependabot.CheckPassing, true},
		{dependabot.CheckPending, true},
		{dependabot.CheckFailing, false},
	}
	for _, tt := range tests {
		t.Run(checksStatus(tt.checks), func(t *testing.T) {
			if got := defaultApprove(dependabot.PullRequest{Checks: tt.checks}); got != tt.want {
				t.Errorf("defaultApprove(%v) = %v, want %v", tt.checks, got, tt.want)
			}
		})
	}
}
