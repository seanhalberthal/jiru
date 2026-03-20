package cli

import (
	"github.com/spf13/cobra"

	"github.com/seanhalberthal/jiru/internal/validate"
)

// GetCmd returns the 'get' subcommand for fetching a single issue as JSON.
func GetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get <issue-key>",
		Short: "Get a Jira issue as JSON",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			key := args[0]
			if err := validate.IssueKey(key); err != nil {
				return err
			}
			issue, err := Client().GetIssue(key)
			if err != nil {
				return err
			}
			return OutputJSON(issue)
		},
	}
}
