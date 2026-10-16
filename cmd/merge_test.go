package cmd

import (
	"errors"
	"strings"
	"testing"
)

func TestMergeCommand(t *testing.T) {
	errMergeFailed := errors.New("exit status 1")

	tests := []struct {
		name         string
		args         []string
		mergeMethods string
		exec         func(args []string) (stdout, stderr string, err error)
		golden       string
		wantErr      string
		want         [][]string
	}{
		{
			name:         "merges every eligible pull request",
			args:         []string{"merge", "--method", "squash", "--delete-branch=true"},
			mergeMethods: "merge_methods_all.json",
			golden:       "merge",
			want: [][]string{
				{"pr", "merge", "1", "--squash", "--delete-branch"},
				{"pr", "merge", "4", "--squash", "--delete-branch"},
			},
		},
		{
			name:         "keeps the branch when asked to",
			args:         []string{"merge", "1", "--method", "merge", "--delete-branch=false"},
			mergeMethods: "merge_methods_all.json",
			golden:       "merge",
			want: [][]string{
				{"pr", "merge", "1", "--merge"},
			},
		},
		{
			name:         "asks dependabot to rebase when the merge commit cannot be created",
			args:         []string{"merge", "1", "--method", "rebase", "--delete-branch=true"},
			mergeMethods: "merge_methods_all.json",
			exec: func(args []string) (string, string, error) {
				if args[1] == "merge" {
					return "", "Pull request is not mergeable: the merge commit cannot be cleanly created.", errMergeFailed
				}
				return "", "", nil
			},
			golden: "merge",
			want: [][]string{
				{"pr", "merge", "1", "--rebase", "--delete-branch"},
				{"pr", "comment", "1", "--body", "@dependabot rebase"},
			},
		},
		{
			name:         "asks dependabot to rebase when the pull request became conflicting",
			args:         []string{"merge", "4", "--method", "squash", "--delete-branch=true"},
			mergeMethods: "merge_methods_all.json",
			exec: func(args []string) (string, string, error) {
				switch args[1] {
				case "merge":
					return "", "GraphQL: Base branch was modified.", errMergeFailed
				case "view":
					return "CONFLICTING\n", "", nil
				}
				return "", "", nil
			},
			golden: "merge",
			want: [][]string{
				{"pr", "merge", "4", "--squash", "--delete-branch"},
				{"pr", "view", "4", "--json", "mergeable", "--jq", ".mergeable"},
				{"pr", "comment", "4", "--body", "@dependabot rebase"},
			},
		},
		{
			name:         "does not ask for a rebase on unrelated failures",
			args:         []string{"merge", "4", "--method", "squash", "--delete-branch=true"},
			mergeMethods: "merge_methods_all.json",
			exec: func(args []string) (string, string, error) {
				switch args[1] {
				case "merge":
					return "", "GraphQL: Required status check is expected.", errMergeFailed
				case "view":
					return "MERGEABLE\n", "", nil
				}
				return "", "", nil
			},
			golden: "merge",
			want: [][]string{
				{"pr", "merge", "4", "--squash", "--delete-branch"},
				{"pr", "view", "4", "--json", "mergeable", "--jq", ".mergeable"},
			},
		},
		{
			name:         "refuses a merge method disabled on the repository",
			args:         []string{"merge", "--method", "merge", "--delete-branch=true"},
			mergeMethods: "merge_methods_squash_only.json",
			golden:       "merge_method_refused",
			wantErr:      `merge method "merge" is not allowed on this repository; allowed: squash`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fake := newFakeGitHub(t, map[string]string{
				"RepoMergeMethods": tt.mergeMethods,
				"DependabotPRs":    "pull_requests.json",
			})
			fake.exec = tt.exec

			err := runCommand(t, tt.args...)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("merge error = %v, want %q", err, tt.wantErr)
				}
			} else if err != nil {
				t.Fatalf("merge error: %v", err)
			}

			fake.assertRequestsMatchGolden(tt.golden)
			fake.assertExecs(tt.want)
		})
	}
}
