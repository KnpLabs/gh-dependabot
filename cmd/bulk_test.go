package cmd

import (
	"reflect"
	"testing"
)

func TestParsePRNumbers(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		want    []int
		wantErr string
	}{
		{name: "no arguments", args: nil},
		{name: "keeps order", args: []string{"4", "1", "12"}, want: []int{4, 1, 12}},
		{name: "drops duplicates", args: []string{"4", "1", "4"}, want: []int{4, 1}},
		{name: "not a number", args: []string{"4", "abc"}, wantErr: "invalid pull request number: abc"},
		{name: "hash prefix", args: []string{"#4"}, wantErr: "invalid pull request number: #4"},
		{name: "zero", args: []string{"0"}, wantErr: "invalid pull request number: 0"},
		{name: "negative", args: []string{"-3"}, wantErr: "invalid pull request number: -3"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parsePRNumbers(tt.args)
			if tt.wantErr != "" {
				if err == nil || err.Error() != tt.wantErr {
					t.Fatalf("error = %v, want %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parsePRNumbers(%q) = %v, want %v", tt.args, got, tt.want)
			}
		})
	}
}
