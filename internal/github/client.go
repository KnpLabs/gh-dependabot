package github

import (
	"bytes"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/cli/go-gh/v2"
	"github.com/cli/go-gh/v2/pkg/api"
	"github.com/cli/go-gh/v2/pkg/repository"
	"github.com/knplabs/gh-dependabot/internal/dependabot"
)

var ErrConflict = errors.New("merge conflict")

type Client interface {
	ListOpenDependabotPRs() ([]dependabot.PullRequest, error)
	AllowedMergeMethods() (dependabot.AllowedMergeMethods, error)
	Approve(number int) error
	Merge(number int, method dependabot.MergeMethod, deleteBranch bool) error
	RequestRebase(number int) error
	Diff(number int) (string, error)
}

type Exec func(args ...string) (stdout, stderr bytes.Buffer, err error)

type client struct {
	exec    Exec
	graphql *api.GraphQLClient
	repo    repository.Repository
}

func New(exec Exec, graphql *api.GraphQLClient, repo repository.Repository) Client {
	return &client{exec: exec, graphql: graphql, repo: repo}
}

func Default() (Client, error) {
	graphql, err := api.DefaultGraphQLClient()
	if err != nil {
		return nil, fmt.Errorf("failed to create GraphQL client: %w", err)
	}
	repo, err := repository.Current()
	if err != nil {
		return nil, fmt.Errorf("failed to determine current repository: %w", err)
	}
	return New(gh.Exec, graphql, repo), nil
}

func (c *client) Approve(number int) error {
	_, err := c.run("pr", "review", strconv.Itoa(number), "--approve")
	return err
}

func (c *client) Merge(number int, method dependabot.MergeMethod, deleteBranch bool) error {
	args := []string{"pr", "merge", strconv.Itoa(number), method.Flag()}
	if deleteBranch {
		args = append(args, "--delete-branch")
	}
	_, err := c.run(args...)
	if err == nil {
		return nil
	}
	if strings.Contains(err.Error(), "cannot be cleanly created") || c.isConflicting(number) {
		return fmt.Errorf("%w: %s", ErrConflict, err)
	}
	return err
}

func (c *client) isConflicting(number int) bool {
	stdout, err := c.run("pr", "view", strconv.Itoa(number), "--json", "mergeable", "--jq", ".mergeable")
	return err == nil && dependabot.Mergeability(strings.TrimSpace(stdout)) == dependabot.Conflicting
}

func (c *client) RequestRebase(number int) error {
	_, err := c.run("pr", "comment", strconv.Itoa(number), "--body", "@dependabot rebase")
	return err
}

func (c *client) Diff(number int) (string, error) {
	return c.run("pr", "diff", strconv.Itoa(number))
}

func (c *client) run(args ...string) (string, error) {
	stdout, stderr, err := c.exec(args...)
	if err != nil {
		if msg := strings.TrimSpace(stderr.String()); msg != "" {
			return "", errors.New(msg)
		}
		return "", err
	}
	return stdout.String(), nil
}

func (c *client) repoVariables() map[string]interface{} {
	return map[string]interface{}{
		"owner": c.repo.Owner,
		"name":  c.repo.Name,
	}
}
