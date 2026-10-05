# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

A GitHub CLI extension (`gh dependabot`) written in Go to list, approve, merge and interactively review Dependabot pull requests on the repository in the current working directory.

## Commands

```bash
go build -o gh-dependabot .   # build
go vet ./...                  # static checks (no linter config in the repo)
gofmt -l .                    # formatting check
```

There is no test suite yet (`go test ./...` compiles but finds no tests).

To try a change, build then run the binary from inside a repo that has open Dependabot PRs: `/path/to/gh-dependabot/gh-dependabot [interactive|approve|merge]`. It relies on an authenticated `gh` and infers the repo from the cwd, so there is no need to `gh extension install .` between rebuilds.

Releases are cut by pushing a `v*` tag; `.github/workflows/release.yml` uses `cli/gh-extension-precompile` to build binaries with the Go version from `go.mod`.

## Architecture

Single package `cmd/` built on Cobra; `main.go` just calls `cmd.Execute()`. Each subcommand registers itself on `rootCmd` from its file's `init()`.

- **`root.go`**: no subcommand → Bubble Tea table view (`listModel` in `list.go`).
- **`list.go`**: holds the shared PR-fetching and status helpers: `fetchDependabotPullRequests` (GraphQL, returns *all* open Dependabot PRs with title), `mapStateToConclusion`, `checksStatus`, `mergeableStatus`. Also defines `prsLoadedMsg` / `prsErrorMsg`, reused by the interactive TUI.
- **`approve.go`**: holds `listDependabotPRs`, a *second* GraphQL query that returns only **eligible** PRs (all checks SUCCESS/SKIPPED or no checks, and `mergeable == MERGEABLE`), plus `isDependabotPR`, `hasPassingChecks`, `filterByNumbers`. Used by both `approve` and `merge`.
- **`merge.go`**: `merge` command plus `mergeMethodFlag`, `fetchAllowedMergeMethods` and `validateMergeMethod`. The chosen `--method` is checked against the repo's allowed merge settings before anything runs. Also reused by `interactive --merge`.
- **`interactive.go`**: Bubble Tea TUI going through the unfiltered PR list one at a time. It shows the diff in a viewport, and `y`/`n`/`enter` approve or skip (the `enter` default flips to skip when checks are failing). Approve and merge calls run as async `tea.Cmd`s so the user can move to the next PR right away: `pending` counts in-flight requests, and the program only quits once `done && pending == 0`. Stale `diffLoadedMsg`s are dropped by comparing PR numbers. A summary is printed after the program exits.

Key conventions:
- GitHub interaction goes two ways: reads use `go-gh` `api.DefaultGraphQLClient()` + `repository.Current()`, and mutations shell out with `gh.Exec("pr", ...)` (`pr review --approve`, `pr diff`, `pr merge`).
- A PR counts as Dependabot's if the author is `dependabot[bot]` / `app/dependabot` / `dependabot` **or** the head branch starts with `dependabot/`.
- GraphQL fetches the first 100 open PRs only (no pagination). Check contexts mix `CheckRun.conclusion` and `StatusContext.state`; the state is normalised through `mapStateToConclusion`.
- The two GraphQL queries in `list.go` and `approve.go` are near-duplicates: when changing the fields or the Dependabot detection logic, update both.
- Bubble Tea/Bubbles are **v2**, imported from `charm.land/...` (not `github.com/charmbracelet/...`): `View()` returns `tea.View` and keys arrive as `tea.KeyPressMsg`.
