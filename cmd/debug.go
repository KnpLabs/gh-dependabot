package cmd

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/cli/go-gh/v2/pkg/api"
	"github.com/cli/go-gh/v2/pkg/repository"
	"github.com/spf13/cobra"
)

var debugCmd = &cobra.Command{
	Use:   "debug",
	Short: "Print diagnostic information about dependabot PR detection",
	Long:  `Queries the GitHub GraphQL API and prints raw data to help diagnose issues with PR detection.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := api.DefaultGraphQLClient()
		if err != nil {
			return fmt.Errorf("failed to create GraphQL client: %w", err)
		}

		repo, err := repository.Current()
		if err != nil {
			return fmt.Errorf("failed to determine current repository: %w", err)
		}

		fmt.Printf("Resolved repository: %s/%s (host: %s)\n\n", repo.Owner, repo.Name, repo.Host)

		query := `query DependabotPRsDebug($owner: String!, $name: String!) {
			repository(owner: $owner, name: $name) {
				pullRequests(states: OPEN, first: 100) {
					nodes {
						number
						title
						headRefName
						author { login }
						mergeable
					}
				}
			}
		}`

		variables := map[string]interface{}{
			"owner": repo.Owner,
			"name":  repo.Name,
		}

		var result struct {
			Repository struct {
				PullRequests struct {
					Nodes []struct {
						Number      int    `json:"number"`
						Title       string `json:"title"`
						HeadRefName string `json:"headRefName"`
						Author      struct {
							Login string `json:"login"`
						} `json:"author"`
						Mergeable string `json:"mergeable"`
					} `json:"nodes"`
				} `json:"pullRequests"`
			} `json:"repository"`
		}

		if err := client.Do(query, variables, &result); err != nil {
			return fmt.Errorf("GraphQL query failed: %w", err)
		}

		nodes := result.Repository.PullRequests.Nodes
		fmt.Printf("Total open PRs returned by API: %d\n\n", len(nodes))

		if len(nodes) == 0 {
			fmt.Println("No open PRs were returned by the API.")
			fmt.Println("\nPossible causes:")
			fmt.Println("  - The repository has no open PRs")
			fmt.Println("  - The resolved repository is incorrect (check owner/name above)")
			fmt.Println("  - Token permissions issue for private repositories")

			// Try raw JSON to see if there's a marshaling issue
			var rawResult json.RawMessage
			if err := client.Do(query, variables, &rawResult); err != nil {
				fmt.Printf("\nRaw query also failed: %v\n", err)
			} else {
				fmt.Printf("\nRaw API response:\n%s\n", string(rawResult))
			}
			return nil
		}

		fmt.Println("PRs found:")
		fmt.Println(strings.Repeat("-", 100))
		for _, node := range nodes {
			isDep := isDependabotPR(node.Author.Login, node.HeadRefName)
			fmt.Printf("  #%-4d | author=%-20q | branch=%-50q | mergeable=%-10s | isDependabot=%v\n",
				node.Number, node.Author.Login, node.HeadRefName, node.Mergeable, isDep)
		}
		fmt.Println(strings.Repeat("-", 100))

		var matched int
		for _, node := range nodes {
			if isDependabotPR(node.Author.Login, node.HeadRefName) {
				matched++
			}
		}
		fmt.Printf("\nDependabot PRs matched: %d\n", matched)

		return nil
	},
}

func init() {
	rootCmd.AddCommand(debugCmd)
}
