package cmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/cli/go-gh/v2/pkg/api"
)

var update = flag.Bool("update", false, "rewrite golden files in testdata/")

var operationName = regexp.MustCompile(`^\s*(?:query|mutation)\s+(\w+)`)

type graphQLRequest struct {
	Query     string         `json:"query"`
	Variables map[string]any `json:"variables"`
}

type fakeGitHub struct {
	t         *testing.T
	responses map[string]string
	requests  []graphQLRequest
	execs     [][]string
	exec      func(args []string) (stdout, stderr string, err error)
}

func newFakeGitHub(t *testing.T, responses map[string]string) *fakeGitHub {
	t.Helper()
	f := &fakeGitHub{t: t, responses: responses}

	t.Setenv("GH_REPO", "knplabs/fake-repo")

	origClient, origExec := newGraphQLClient, ghExec
	t.Cleanup(func() { newGraphQLClient, ghExec = origClient, origExec })

	newGraphQLClient = func() (*api.GraphQLClient, error) {
		return api.NewGraphQLClient(api.ClientOptions{
			Host:      "github.com",
			AuthToken: "fake-token",
			Transport: f,
		})
	}
	ghExec = func(args ...string) (stdout, stderr bytes.Buffer, err error) {
		f.execs = append(f.execs, args)
		if f.exec == nil {
			return stdout, stderr, nil
		}
		out, errOut, err := f.exec(args)
		stdout.WriteString(out)
		stderr.WriteString(errOut)
		return stdout, stderr, err
	}

	return f
}

func (f *fakeGitHub) RoundTrip(req *http.Request) (*http.Response, error) {
	var body graphQLRequest
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		return nil, fmt.Errorf("decoding GraphQL request: %w", err)
	}
	f.requests = append(f.requests, body)

	match := operationName.FindStringSubmatch(body.Query)
	if match == nil {
		return nil, errors.New("GraphQL request has no operation name")
	}
	fixture, ok := f.responses[match[1]]
	if !ok {
		return nil, fmt.Errorf("no fixture registered for operation %s", match[1])
	}
	data, err := os.ReadFile(filepath.Join("testdata", "graphql", fixture))
	if err != nil {
		return nil, err
	}

	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": {"application/json"}},
		Body:       io.NopCloser(bytes.NewReader(data)),
		Request:    req,
	}, nil
}

func (f *fakeGitHub) assertRequestsMatchGolden(name string) {
	f.t.Helper()

	var b strings.Builder
	for i, r := range f.requests {
		if i > 0 {
			b.WriteString("\n---\n\n")
		}
		variables, err := json.Marshal(r.Variables)
		if err != nil {
			f.t.Fatalf("encoding variables: %v", err)
		}
		fmt.Fprintf(&b, "%s\n\nvariables: %s\n", r.Query, variables)
	}
	got := b.String()

	path := filepath.Join("testdata", "golden", name+".graphql")
	if *update {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			f.t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			f.t.Fatal(err)
		}
		return
	}

	want, err := os.ReadFile(path)
	if err != nil {
		f.t.Fatalf("reading golden file (run `go test ./cmd -update` to create it): %v", err)
	}
	if got != string(want) {
		f.t.Errorf("GraphQL requests differ from %s (run `go test ./cmd -update` if the change is intended)\n--- got ---\n%s\n--- want ---\n%s", path, got, want)
	}
}

func (f *fakeGitHub) assertExecs(want [][]string) {
	f.t.Helper()
	if len(want) == 0 && len(f.execs) == 0 {
		return
	}
	if !reflect.DeepEqual(f.execs, want) {
		f.t.Errorf("gh calls mismatch\n got: %q\nwant: %q", f.execs, want)
	}
}

func runCommand(t *testing.T, args ...string) error {
	t.Helper()
	rootCmd.SetArgs(args)
	rootCmd.SetOut(io.Discard)
	rootCmd.SetErr(io.Discard)
	t.Cleanup(func() {
		rootCmd.SetArgs(nil)
		rootCmd.SetOut(nil)
		rootCmd.SetErr(nil)
	})
	return rootCmd.Execute()
}
