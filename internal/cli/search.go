package cli

import (
	"github.com/spf13/cobra"
)

// SearchCmd returns the 'search' subcommand for JQL search with JSON output.
func SearchCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "search <jql>",
		Short: "Search Jira issues using JQL and output as JSON",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			issues, err := Client().SearchJQL(args[0], 0)
			if err != nil {
				return err
			}
			return OutputJSON(issues)
		},
	}
}
