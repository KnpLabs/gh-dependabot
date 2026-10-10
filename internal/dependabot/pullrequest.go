package dependabot

import "strings"

type CheckState int

const (
	CheckNone CheckState = iota
	CheckPassing
	CheckFailing
	CheckPending
)

type Mergeability string

const (
	Mergeable   Mergeability = "MERGEABLE"
	Conflicting Mergeability = "CONFLICTING"
	Unknown     Mergeability = "UNKNOWN"
)

type PullRequest struct {
	Number      int
	Title       string
	Author      string
	HeadRefName string
	Checks      CheckState
	Mergeable   Mergeability
}

func CheckStateOf(conclusions []string) CheckState {
	if len(conclusions) == 0 {
		return CheckNone
	}
	allPass := true
	hasFail := false
	for _, c := range conclusions {
		switch c {
		case "SUCCESS", "SKIPPED":
		case "FAILURE", "ERROR":
			hasFail = true
			allPass = false
		default:
			allPass = false
		}
	}
	if allPass {
		return CheckPassing
	}
	if hasFail {
		return CheckFailing
	}
	return CheckPending
}

func (s CheckState) Passed() bool {
	return s == CheckNone || s == CheckPassing
}

func (pr PullRequest) IsDependabot() bool {
	return isDependabotAuthor(pr.Author) || strings.HasPrefix(pr.HeadRefName, "dependabot/")
}

func isDependabotAuthor(login string) bool {
	return login == "app/dependabot" || login == "dependabot[bot]" || login == "dependabot"
}

func (pr PullRequest) Eligible() bool {
	return pr.Checks.Passed() && pr.Mergeable == Mergeable
}

func FilterByNumbers(prs []PullRequest, numbers []int) []PullRequest {
	numSet := make(map[int]bool)
	for _, n := range numbers {
		numSet[n] = true
	}

	var filtered []PullRequest
	for _, pr := range prs {
		if numSet[pr.Number] {
			filtered = append(filtered, pr)
		}
	}
	return filtered
}
