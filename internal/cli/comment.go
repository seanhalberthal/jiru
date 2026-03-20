package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/seanhalberthal/jiru/internal/validate"
)

// CommentCmd returns the 'comment' subcommand for adding a comment to an issue.
func CommentCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "comment <issue-key> <body>",
		Short: "Add a comment to a Jira issue",
		Long: `Post a comment on an issue.

Body accepts:
  jiru comment PROJ-123 "Comment text"   Literal string
  jiru comment PROJ-123 -                Read from stdin
  jiru comment PROJ-123 @file.md         Read from file`,
		Args: cobra.ExactArgs(2),
		RunE: func(_ *cobra.Command, args []string) error {
			key := args[0]
			if err := validate.IssueKey(key); err != nil {
				return err
			}

			body, err := resolveInput(args[1])
			if err != nil {
				return fmt.Errorf("reading comment body: %w", err)
			}
			if body == "" {
				return fmt.Errorf("comment body cannot be empty")
			}

			if err := Client().AddComment(key, body); err != nil {
				return fmt.Errorf("adding comment: %w", err)
			}

			return OutputJSON(map[string]any{
				"ok":  true,
				"key": key,
			})
		},
	}
}
