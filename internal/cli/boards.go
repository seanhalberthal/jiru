package cli

import (
	"github.com/spf13/cobra"
)

// BoardsCmd returns the 'boards' subcommand for listing boards as JSON.
func BoardsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "boards",
		Short: "List Jira boards as JSON",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			boards, err := Client().Boards(Config().Project)
			if err != nil {
				return err
			}
			return OutputJSON(boards)
		},
	}
}
