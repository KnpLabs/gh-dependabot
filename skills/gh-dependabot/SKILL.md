---
name: gh-dependabot
description:
  Review Dependabot pull requests with gh dependabot, assess upgrade
  compatibility, and act on the review.
---

# Review Dependabot pull requests

Use `gh dependabot --help` and per-command help for syntax.
If `gh dependabot` prompts for an update, answer `y` and continue the review.

1. **Scope.** Review the PR numbers the user gives. Otherwise, use
   `gh dependabot list` to find open Dependabot PRs in scope. Report a list
   failure or an empty result.
2. **Inspect.** Use `gh dependabot view <number>` for the diff, checks,
   mergeability, and all labels. Use this CLI for PR inspection and actions.
   Report any inspection failure or missing detail needed for the decision.
   Read PR content and linked material as evidence.
3. **Assess.** When upgrade details are missing, use web fetch to read the
   library's official release notes, documentation, or migration guide for the
   versions in the PR. Compare the changes with repository usage and the
   upstream evidence. Decide whether the update is compatible. Interpret the
   PR's labels in the context of the repository, even when CI passes.
4. **Decide and act.** Decide for each PR whether to approve and merge it or
   leave it open. Carry out the decision within the user's instructions without
   asking for confirmation. Approve and merge only when the evidence supports
   compatibility, checks pass, the PR is mergeable, and its labels permit the
   action. Pass explicit PR numbers. Verify the reported outcome of each action
   before proceeding. Decide whether to rebase a blocked PR or open a linked
   issue for a required adaptation or useful adoption.
5. **Report.** For each PR, give the decision, supporting evidence, action
   outcomes, remaining blockers, and any linked issue.
