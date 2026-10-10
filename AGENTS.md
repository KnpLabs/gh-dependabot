# AGENTS.md

This file provides guidance to coding agents when working with code in this repository.

## What this is

A GitHub CLI extension (`gh dependabot`) written in Go to list, approve, merge and interactively review Dependabot pull requests on the repository in the current working directory.

## Commands

Prefer the `Makefile` targets over calling `go` directly (run `make` to list them):

```bash
make build   # build the gh-dependabot binary
make fmt     # format the code (gofmt -w)
make lint    # gofmt check + go vet (no linter config in the repo)
make test    # go test -race -cover ./...
make check   # lint + test + build, run before committing
make tidy    # go mod tidy
make clean   # remove the built binary
```

Tests are table-driven, stdlib `testing` only, next to the code (`cmd/*_test.go`, `internal/*/*_test.go`), with fixtures in the sibling `testdata/` directories.

`internal/github` is tested on its own with a fake `exec` (exact `gh` args, stderr → error, conflict → `ErrConflict`) and a fake `http.RoundTripper` serving GraphQL fixtures from `internal/github/testdata/`. The queries sent are snapshotted in `internal/github/testdata/golden/*.graphql`; after an intended query change, regenerate them with `go test ./internal/github -update` and review the diff.

Commands are tested through a fake `github.Client` (`cmd/harness_test.go`): `newFakeClient` swaps the `newClient` package variable from `root.go` and records every call, and `runCommand` captures stdout/stderr and resets flags between runs. Commands must therefore get GitHub access only through `newClient()` and write only through `cmd.OutOrStdout()` / `cmd.ErrOrStderr()`, never `fmt.Print*` / `os.Stdout`. The `interactive` TUI is tested without a terminal (`cmd/interactive_model_test.go`): `tuiDriver` feeds messages to `Update`, runs the returned `tea.Cmd`s synchronously against the fake client (expanding `tea.BatchMsg`, dropping spinner ticks, recording `tea.QuitMsg`), and `keyPress` builds `tea.KeyPressMsg` values directly. Hold the messages returned by `press` instead of `settle`-ing them to simulate requests still in flight. Don't use `teatest`. The summary is snapshotted in `cmd/testdata/golden/summary/*.txt` (`go test ./cmd -update`).

To try a change, `make build` then run the binary from inside a repo that has open Dependabot PRs: `/path/to/gh-dependabot/gh-dependabot [interactive|approve|merge]`. It relies on an authenticated `gh` and infers the repo from the cwd, so there is no need to `gh extension install .` between rebuilds.

Releases are cut by pushing a `v*` tag; `.github/workflows/release.yml` uses `cli/gh-extension-precompile` to build binaries with the Go version from `go.mod`.

## Architecture

`cmd/` holds the Cobra wiring and the TUIs (`interactive.go` is the command, with its model/`Update` in `interactive_model.go`, the `tea.Cmd`s calling the client in `interactive_commands.go` and rendering plus summary in `interactive_view.go`); `main.go` just calls `cmd.Execute()`. Each subcommand registers itself on `rootCmd` from its file's `init()`.

`internal/dependabot/` is the pure domain (no I/O): `PullRequest`, `CheckState`, `Mergeability`, eligibility rules and `MergeMethod` parsing/validation. Business rules live there and must never depend on display strings; `checksStatus` / `mergeableStatus` in `cmd/list.go` only render them.

Key conventions:
- `approve` and `merge` share their plumbing in `cmd/bulk.go`: `parsePRNumbers`, `resolveMergeMethod` (parse + check against the repo's allowed methods, also used by `interactive --merge`) and `runOnEligiblePRs`, the list → filter → act loop driven by a `bulkAction`.
- All GitHub I/O lives behind the `github.Client` interface (`internal/github`). Reads go through the GraphQL client, mutations shell out to `gh pr ...` through the injected `exec`, and every `gh` call goes through `run()`, which turns stderr into the error message. `Merge` returns an error wrapping `github.ErrConflict` when the PR conflicts, and callers decide whether to `RequestRebase`.
- PRs are fetched by a single query, `ListOpenDependabotPRs` (`internal/github/pullrequests.go`), shared by every command.
- A PR counts as Dependabot's if the author is `dependabot[bot]` / `app/dependabot` / `dependabot` **or** the head branch starts with `dependabot/`.
- GraphQL open PRs only. Check contexts mix `CheckRun.conclusion` and `StatusContext.state`; the state is normalised through `mapStateToConclusion`.
- Bubble Tea/Bubbles are **v2**, imported from `charm.land/...` (not `github.com/charmbracelet/...`): `View()` returns `tea.View` and keys arrive as `tea.KeyPressMsg`.

## Code style

Don't add comments to the code (doc comments included) unless the user explicitly asks for them. Rely on clear names instead. Leave existing comments alone.
