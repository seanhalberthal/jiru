package cli

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/seanhalberthal/jiru/internal/client"
	"github.com/seanhalberthal/jiru/internal/validate"
)

// EditCmd returns the 'edit' subcommand for updating issue fields.
func EditCmd() *cobra.Command {
	var (
		summary     string
		description string
		priority    string
		assign      string
	)

	cmd := &cobra.Command{
		Use:   "edit <issue-key>",
		Short: "Edit a Jira issue",
		Long: `Update issue fields. At least one flag is required.

Description accepts:
  --description "text"    Literal string
  --description -         Read from stdin
  --description @file.md  Read from file`,
		Args: cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			key := args[0]
			if err := validate.IssueKey(key); err != nil {
				return err
			}

			if summary == "" && description == "" && priority == "" && assign == "" {
				return fmt.Errorf("at least one of --summary, --description, --priority, or --assign is required")
			}

			c := Client()

			// Handle description input sources.
			desc, err := resolveInput(description)
			if err != nil {
				return fmt.Errorf("reading description: %w", err)
			}

			// Handle assignment separately.
			if assign != "" {
				if err := c.AssignIssue(key, assign); err != nil {
					return fmt.Errorf("assigning issue: %w", err)
				}
			}

			// Edit fields if any are set.
			if summary != "" || desc != "" || priority != "" {
				req := &client.EditIssueRequest{
					Summary:     summary,
					Description: desc,
					Priority:    priority,
				}
				if err := c.EditIssue(key, req); err != nil {
					return fmt.Errorf("editing issue: %w", err)
				}
			}

			// Fetch and return the updated issue.
			issue, err := c.GetIssue(key)
			if err != nil {
				return err
			}
			return OutputJSON(issue)
		},
	}

	cmd.Flags().StringVar(&summary, "summary", "", "new issue summary")
	cmd.Flags().StringVar(&description, "description", "", `new description (use "-" for stdin, "@file" for file)`)
	cmd.Flags().StringVar(&priority, "priority", "", "new priority name")
	cmd.Flags().StringVar(&assign, "assign", "", "assignee account ID or email")

	return cmd
}

// resolveInput reads content from a flag value:
//   - empty string: returns empty (no change)
//   - "-": reads from stdin
//   - "@path": reads from file
//   - anything else: returns the literal string
func resolveInput(value string) (string, error) {
	if value == "" {
		return "", nil
	}
	if value == "-" {
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			return "", err
		}
		return string(data), nil
	}
	if strings.HasPrefix(value, "@") {
		data, err := os.ReadFile(value[1:])
		if err != nil {
			return "", err
		}
		return string(data), nil
	}
	return value, nil
}
