package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/seanhalberthal/jiru/internal/jira"
)

// ListCmd returns the 'list' subcommand for listing sprint/board issues as JSON.
func ListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List issues from the configured board's active sprint as JSON",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			cfg := Config()
			if cfg.BoardID == 0 {
				return fmt.Errorf("no board configured; run 'jiru' to select a board or set JIRA_BOARD_ID")
			}

			c := Client()

			// Try active sprint first.
			sprints, err := c.BoardSprints(cfg.BoardID, "active")
			if err != nil {
				return err
			}

			var issues []jira.Issue
			if len(sprints) > 0 {
				issues, err = c.SprintIssues(sprints[0].ID)
				if err != nil {
					return err
				}
			} else {
				// No active sprint — fall back to all board issues.
				issues, err = c.BoardIssues(cfg.Project)
				if err != nil {
					return err
				}
			}

			return OutputJSON(issues)
		},
	}
}
