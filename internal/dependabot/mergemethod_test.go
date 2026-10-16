package dependabot

import "testing"

func TestParseMergeMethod(t *testing.T) {
	tests := []struct {
		method   string
		want     MergeMethod
		wantFlag string
		wantErr  string
	}{
		{"merge", MergeMethodMerge, "--merge", ""},
		{"rebase", MergeMethodRebase, "--rebase", ""},
		{"squash", MergeMethodSquash, "--squash", ""},
		{"", "", "", `invalid merge method "": must be one of merge, rebase, squash`},
		{"Squash", "", "", `invalid merge method "Squash": must be one of merge, rebase, squash`},
		{"fast-forward", "", "", `invalid merge method "fast-forward": must be one of merge, rebase, squash`},
	}
	for _, tt := range tests {
		t.Run(tt.method, func(t *testing.T) {
			got, err := ParseMergeMethod(tt.method)
			assertError(t, err, tt.wantErr)
			if got != tt.want {
				t.Errorf("ParseMergeMethod(%q) = %q, want %q", tt.method, got, tt.want)
			}
			if err == nil && got.Flag() != tt.wantFlag {
				t.Errorf("ParseMergeMethod(%q).Flag() = %q, want %q", tt.method, got.Flag(), tt.wantFlag)
			}
		})
	}
}

func TestValidate(t *testing.T) {
	all := AllowedMergeMethods{Merge: true, Squash: true, Rebase: true}

	tests := []struct {
		name    string
		method  MergeMethod
		allowed AllowedMergeMethods
		wantErr string
	}{
		{"merge allowed", MergeMethodMerge, all, ""},
		{"squash allowed", MergeMethodSquash, all, ""},
		{"rebase allowed", MergeMethodRebase, all, ""},
		{"only squash enabled", MergeMethodSquash, AllowedMergeMethods{Squash: true}, ""},
		{
			"invalid method",
			"octopus", all,
			`invalid merge method "octopus": must be one of merge, rebase, squash`,
		},
		{
			"invalid method wins over nothing enabled",
			"octopus", AllowedMergeMethods{},
			`invalid merge method "octopus": must be one of merge, rebase, squash`,
		},
		{
			"merge disabled, one other enabled",
			MergeMethodMerge, AllowedMergeMethods{Squash: true},
			`merge method "merge" is not allowed on this repository; allowed: squash`,
		},
		{
			"rebase disabled, others listed in order",
			MergeMethodRebase, AllowedMergeMethods{Merge: true, Squash: true},
			`merge method "rebase" is not allowed on this repository; allowed: merge, squash`,
		},
		{
			"nothing enabled",
			MergeMethodSquash, AllowedMergeMethods{},
			`merge method "squash" is not allowed on this repository (no merge methods are enabled)`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertError(t, tt.allowed.Validate(tt.method), tt.wantErr)
		})
	}
}

func assertError(t *testing.T, err error, want string) {
	t.Helper()
	if want == "" {
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		return
	}
	if err == nil {
		t.Errorf("expected error %q, got nil", want)
		return
	}
	if err.Error() != want {
		t.Errorf("error = %q, want %q", err.Error(), want)
	}
}
