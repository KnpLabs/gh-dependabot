package cmd

import "testing"

func TestApproveCommand(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want [][]string
	}{
		{
			name: "approves every eligible pull request",
			args: []string{"approve"},
			want: [][]string{
				{"pr", "review", "1", "--approve"},
				{"pr", "review", "4", "--approve"},
			},
		},
		{
			name: "approves only the requested pull requests",
			args: []string{"approve", "4", "5"},
			want: [][]string{
				{"pr", "review", "4", "--approve"},
			},
		},
		{
			name: "skips pull requests that are not eligible",
			args: []string{"approve", "3"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fake := newFakeGitHub(t, map[string]string{"DependabotPRs": "pull_requests.json"})

			if err := runCommand(t, tt.args...); err != nil {
				t.Fatalf("approve error: %v", err)
			}

			fake.assertRequestsMatchGolden("approve")
			fake.assertExecs(tt.want)
		})
	}
}
