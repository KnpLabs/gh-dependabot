package cmd

import (
	"errors"
	"fmt"
	"os"
	"reflect"
	"slices"
	"strings"
	"testing"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
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
	client := &fakeClient{prs: reviewQueue}
	d := &tuiDriver{t: t, model: newInteractiveModel(client), client: client}
	if view := d.model.View(); view.AltScreen {
		t.Fatal("opened alternate screen while loading the PR list")
	}
	firstDiff := d.send(fetchDependabotPRsCmd(client)())
	if view := d.model.View(); view.AltScreen || !strings.Contains(view.Content, "Loading diff for PR #1") {
		t.Fatalf("expected inline loading message, got %+v", view)
	}
	d.settle(firstDiff...)

	if view := d.model.View(); !view.AltScreen || d.model.loadingDiff || !strings.Contains(ansi.Strip(view.Content), "+ go 1.1") {
		t.Errorf("want the diff of #1 in the alternate screen, got:\n%s", view.Content)
	}
	d.client.assertCalls(t, []string{"ListOpenDependabotPRs", "Diff 1"})
}

func TestInteractiveDiffFiles(t *testing.T) {
	patch, err := os.ReadFile("testdata/dependency.diff")
	if err != nil {
		t.Fatal(err)
	}
	client := &fakeClient{prs: reviewQueue, diffs: map[int]string{1: string(patch)}}
	d := &tuiDriver{t: t, model: newInteractiveModel(client), client: client}
	d.settle(d.run(d.model.Init())...)

	assertView := func(want ...string) {
		t.Helper()
		view := ansi.Strip(d.model.View().Content)
		for _, line := range want {
			if !strings.Contains(view, line) {
				t.Errorf("view missing %q:\n%s", line, view)
			}
		}
	}
	assertView("File 1/2  go.mod", " 9 10 │   require (", "10    │ -     example.org/lib v1.0.0", "   11 │ +     example.org/lib v1.1.0", "30 31 │   )", "[ ]  Change file", "Enter / y  Approve")
	d.settle(d.press("[")...)
	assertView("File 1/2  go.mod")

	d.settle(d.send(tea.WindowSizeMsg{Width: 80, Height: 10})...)
	d.model.viewport.SetYOffset(4)
	if d.model.viewport.YOffset() == 0 {
		t.Fatal("expected scroll offset within the first file")
	}
	assertView("File 1/2  go.mod")
	d.settle(d.press("]")...)
	assertView("File 2/2  go.sum", "1   │ - example.org/lib v1.0.0")
	if got := d.model.viewport.YOffset(); got != 0 {
		t.Errorf("scroll offset after changing files = %d, want 0", got)
	}
	d.settle(d.press("]")...)
	assertView("File 2/2  go.sum")
	d.settle(d.press("[")...)
	assertView("File 1/2  go.mod")
	d.settle(d.press("]")...)
	previous := d.model.View().Content
	previousBody := d.model.diffView()
	nextDiff := d.press("n")
	assertView("File 2/2  go.sum", "PR 1 of 2 — #1", "Loading diff for PR #5")
	loading := d.model.View()
	if !loading.AltScreen || !strings.HasPrefix(loading.Content, previousBody+"\n") {
		t.Error("the previous diff should stay visible while the next one loads")
	}
	if strings.Count(loading.Content, "\n") != strings.Count(previous, "\n") || strings.Contains(ansi.Strip(loading.Content), "Enter / y  Approve") {
		t.Error("the loading footer should replace the approval prompt and retain its height")
	}
	d.settle(d.press("y")...)
	d.settle(nextDiff...)
	assertView("File 1/1  go.mod", "+ go 1.5", "Enter / n  Skip")
	client.assertCalls(t, []string{"ListOpenDependabotPRs", "Diff 1", "Diff 5"})
}

func TestInteractiveSplitLayout(t *testing.T) {
	patch, err := os.ReadFile("testdata/dependency.diff")
	if err != nil {
		t.Fatal(err)
	}
	client := &fakeClient{prs: reviewQueue, diffs: map[int]string{1: string(patch)}}
	d := &tuiDriver{t: t, model: newInteractiveModel(client), client: client}
	d.settle(d.run(d.model.Init())...)

	assertView := func(width int, want ...string) {
		t.Helper()
		view := ansi.Strip(d.model.View().Content)
		for _, line := range want {
			if !strings.Contains(view, line) {
				t.Errorf("view missing %q:\n%s", line, view)
			}
		}
		for _, line := range strings.Split(view, "\n") {
			if ansi.StringWidth(line) > width {
				t.Errorf("line exceeds terminal width %d: %q", width, line)
			}
		}
	}
	assertPaired := func(before, after string) {
		t.Helper()
		rows := strings.Split(ansi.Strip(d.model.viewport.GetContent()), "\n")
		if !slices.ContainsFunc(rows, func(line string) bool {
			return strings.Contains(line, before) && strings.Contains(line, after)
		}) {
			t.Errorf("expected %q and %q on the same row:\n%s", before, after, strings.Join(rows, "\n"))
		}
	}
	d.settle(d.send(tea.WindowSizeMsg{Width: 132, Height: 20})...)
	assertView(132, "Files  1/2", "▎M go.mod", "M go.sum", "+3 -2", "v  Switch view")
	assertPaired("9   require (", "10   require (")
	assertPaired("example.org/lib v1.0.0", "example.org/lib v1.1.0")

	d.settle(d.press("v")...)
	assertView(132, "10    │ -     example.org/lib v1.0.0", "   11 │ +     example.org/lib v1.1.0")
	d.settle(d.press("v")...)
	d.settle(d.press("]")...)
	assertView(132, "Files  2/2", "▎M go.sum", "+1 -1")
	d.settle(d.send(tea.WindowSizeMsg{Width: 98, Height: 20})...)
	assertView(98, "File 2/2  go.sum", "1   │ - example.org/lib v1.0.0")
	d.settle(d.send(tea.WindowSizeMsg{Width: 132, Height: 20})...)
	assertView(132, "Files  2/2", "▎M go.sum")
	assertPaired("example.org/lib v1.0.0", "example.org/lib v1.1.0")
	client.assertCalls(t, []string{"ListOpenDependabotPRs", "Diff 1"})
}

func TestInteractiveSplitWrapsLongLines(t *testing.T) {
	before := strings.Repeat("旧", 48)
	after := strings.Repeat("新", 48)
	patch := fmt.Sprintf("diff --git a/go.sum b/go.sum\n--- a/go.sum\n+++ b/go.sum\n@@ -1 +1 @@\n-%s\n+%s\n", before, after)
	client := &fakeClient{prs: reviewQueue, diffs: map[int]string{1: patch}}
	d := &tuiDriver{t: t, model: newInteractiveModel(client), client: client}
	d.settle(d.run(d.model.Init())...)
	d.settle(d.send(tea.WindowSizeMsg{Width: 100, Height: 20})...)

	content := ansi.Strip(d.model.viewport.GetContent())
	for _, text := range []string{"旧", "新"} {
		if got := strings.Count(content, text); got != 48 {
			t.Errorf("expected all 48 occurrences of %s, got %d", text, got)
		}
	}
	for _, row := range strings.Split(content, "\n") {
		if ansi.StringWidth(row) > d.model.viewport.Width() {
			t.Errorf("split row exceeds diff pane width: %q", row)
		}
	}
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
	previous := d.model.viewport.GetContent()

	nextDiff := d.press("n")
	if d.model.index != 1 || !d.model.loadingDiff {
		t.Fatalf("index = %d, loadingDiff = %v, want the diff of #5 loading", d.model.index, d.model.loadingDiff)
	}

	d.settle(d.run(fetchDiffCmd(d.client, 1))...)
	if !d.model.loadingDiff || d.model.viewport.GetContent() != previous {
		t.Error("a stale response should leave the previous diff and loading state intact")
	}

	d.settle(nextDiff...)
	if view := ansi.Strip(d.model.View().Content); d.model.loadingDiff || !strings.Contains(view, "+ go 1.5") {
		t.Errorf("want the diff of #5 in the view, got:\n%s", view)
	}
}

func TestInteractiveDiffError(t *testing.T) {
	d := newTUIDriver(t, reviewQueue, nil)

	d.settle(d.press("n")...)
	d.settle(diffLoadedMsg{number: 5, err: errors.New("could not find pull request")})

	if view := d.model.View().Content; !strings.Contains(view, "Failed to load diff: could not find pull request") {
		t.Errorf("missing diff error in the view:\n%s", view)
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
