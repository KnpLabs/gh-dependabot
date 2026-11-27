# Use the CLI for coding-agent review of Dependabot PRs

Codex and Claude Code will use the same CLI as human operators in V1. Plain-text
output lets both agents inspect PRs through their shell without a terminal UI or
a separate integration. The CLI presents GitHub data and enforces its existing
action eligibility rules, while the agent evaluates update compatibility and
acts within the scope authorized by the user.

## CLI interface

- `gh dependabot --help` and per-command help expose the available commands and
  options.
- `gh dependabot list` prints one row per open Dependabot PR in the current
  repository, with the PR number, title, check status, and mergeability.
- `gh dependabot view <number>` prints the title, full PR body, check status,
  mergeability, and unified diff for a single Dependabot PR. Both read commands
  write plain text to standard output and exit.
- The README documents the workflow and existing commands. The agent passes the
  PR number to `approve`, `merge`, or `rebase` when it decides PR by PR. To skip
  a PR, it records its reason and moves on.

## Agent review

The agent uses the same review for every update because a dependency's version
numbers may not reliably signal its impact. It reads the PR body, diff,
changelog, documentation, and migration guide when available. It checks the
reported fixes, test results, and affected usages in the repository, and looks
for concrete opportunities to adopt new features.

| Finding                                                | Decision                                                                                                    |
| ------------------------------------------------------ | ----------------------------------------------------------------------------------------------------------- |
| The update is compatible and tests pass                | Approve and merge when authorized and eligible. In an analysis-only task, report the recommendation.        |
| A code adaptation is required                          | Leave the Dependabot PR open and open a linked upgrade issue when authorized.                               |
| A new feature would be useful and adoption is optional | Open a linked upgrade issue when authorized. The compatible update remains eligible for approval and merge. |
| Checks fail or the PR conflicts                        | Request a rebase when it is relevant, eligible, and authorized. Otherwise, report the blocker.              |
| Compatibility is uncertain                             | Leave the PR open and explain what remains to be verified.                                                  |

An upgrade issue documents the affected usages, the proposed changes or expected
benefit, and the tests to run. Issues are the deliverable for follow-up work,
including small adaptations. At the end of the review, the agent reports its
decision, rationale, action outcome, and any upgrade issue link for each PR.
