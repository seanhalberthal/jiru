package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/seanhalberthal/jiru/internal/validate"
)

// TransitionCmd returns the 'transition' subcommand for listing or executing transitions.
func TransitionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "transition <issue-key> [transition-id]",
		Short: "List or execute issue status transitions",
		Long: `Without a transition ID, lists available transitions as JSON.
With a transition ID, executes the transition.`,
		Args: cobra.RangeArgs(1, 2),
		RunE: func(_ *cobra.Command, args []string) error {
			key := args[0]
			if err := validate.IssueKey(key); err != nil {
				return err
			}

			c := Client()

			// List mode: no transition ID provided.
			if len(args) == 1 {
				transitions, err := c.Transitions(key)
				if err != nil {
					return fmt.Errorf("fetching transitions: %w", err)
				}
				return OutputJSON(transitions)
			}

			// Execute mode.
			transitionID := args[1]
			if err := c.TransitionIssue(key, transitionID); err != nil {
				return fmt.Errorf("transitioning issue: %w", err)
			}

			// Fetch the updated issue to get the new status.
			issue, err := c.GetIssue(key)
			if err != nil {
				return OutputJSON(map[string]any{
					"ok":     true,
					"key":    key,
					"status": "unknown",
				})
			}

			return OutputJSON(map[string]any{
				"ok":     true,
				"key":    key,
				"status": issue.Status,
			})
		},
	}
}
