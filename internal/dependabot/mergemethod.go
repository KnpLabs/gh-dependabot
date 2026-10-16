package dependabot

import (
	"fmt"
	"strings"
)

type MergeMethod string

const (
	MergeMethodMerge  MergeMethod = "merge"
	MergeMethodSquash MergeMethod = "squash"
	MergeMethodRebase MergeMethod = "rebase"
)

type AllowedMergeMethods struct {
	Merge  bool
	Squash bool
	Rebase bool
}

func ParseMergeMethod(s string) (MergeMethod, error) {
	switch m := MergeMethod(s); m {
	case MergeMethodMerge, MergeMethodSquash, MergeMethodRebase:
		return m, nil
	default:
		return "", fmt.Errorf("invalid merge method %q: must be one of merge, rebase, squash", s)
	}
}

func (m MergeMethod) Flag() string {
	return "--" + string(m)
}

func (a AllowedMergeMethods) Validate(m MergeMethod) error {
	if _, err := ParseMergeMethod(string(m)); err != nil {
		return err
	}
	if a.allows(m) {
		return nil
	}

	var enabled []string
	for _, candidate := range []MergeMethod{MergeMethodMerge, MergeMethodSquash, MergeMethodRebase} {
		if a.allows(candidate) {
			enabled = append(enabled, string(candidate))
		}
	}
	if len(enabled) == 0 {
		return fmt.Errorf("merge method %q is not allowed on this repository (no merge methods are enabled)", m)
	}
	return fmt.Errorf("merge method %q is not allowed on this repository; allowed: %s", m, strings.Join(enabled, ", "))
}

func (a AllowedMergeMethods) allows(m MergeMethod) bool {
	switch m {
	case MergeMethodMerge:
		return a.Merge
	case MergeMethodSquash:
		return a.Squash
	case MergeMethodRebase:
		return a.Rebase
	default:
		return false
	}
}
