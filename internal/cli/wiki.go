package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/seanhalberthal/jiru/internal/adf"
)

// WikiCmd returns the 'wiki' command group for Confluence operations.
func WikiCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "wiki",
		Short: "Confluence wiki commands",
	}

	cmd.AddCommand(wikiSpacesCmd(), wikiPageCmd(), wikiEditCmd())
	return cmd
}

func wikiSpacesCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "spaces",
		Short: "List Confluence spaces as JSON",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			spaces, err := Client().ConfluenceSpaces()
			if err != nil {
				return err
			}
			return OutputJSON(spaces)
		},
	}
}

func wikiPageCmd() *cobra.Command {
	var format string

	cmd := &cobra.Command{
		Use:   "page <page-id>",
		Short: "Fetch a Confluence page",
		Long: `Fetch a Confluence page. Output format controlled by --format:
  json       Full page as JSON (default)
  markdown   ADF body converted to Markdown
  adf        Raw ADF JSON body only`,
		Args: cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			page, err := Client().ConfluencePage(args[0])
			if err != nil {
				return err
			}

			switch format {
			case "markdown", "md":
				md, err := adf.ToMarkdown(page.BodyADF)
				if err != nil {
					return fmt.Errorf("converting ADF to markdown: %w", err)
				}
				fmt.Print(md)
				return nil
			case "adf":
				fmt.Println(page.BodyADF)
				return nil
			default:
				return OutputJSON(page)
			}
		},
	}

	cmd.Flags().StringVar(&format, "format", "json", "output format: json, markdown, adf")
	return cmd
}

func wikiEditCmd() *cobra.Command {
	var (
		title  string
		body   string
		format string
	)

	cmd := &cobra.Command{
		Use:   "edit <page-id>",
		Short: "Update a Confluence page",
		Long: `Update a Confluence page's title and/or body.

Body accepts:
  --body "text"     Literal string (interpreted per --format)
  --body -          Read from stdin
  --body @file.md   Read from file

Format controls how the body is interpreted:
  markdown   Convert Markdown to ADF before sending (default)
  adf        Send raw ADF JSON directly`,
		Args: cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			pageID := args[0]
			c := Client()

			if title == "" && body == "" {
				return fmt.Errorf("at least one of --title or --body is required")
			}

			// Fetch current page for version and fallback title.
			current, err := c.ConfluencePage(pageID)
			if err != nil {
				return fmt.Errorf("fetching current page: %w", err)
			}

			pageTitle := title
			if pageTitle == "" {
				pageTitle = current.Title
			}

			bodyADF := current.BodyADF
			if body != "" {
				content, err := resolveInput(body)
				if err != nil {
					return fmt.Errorf("reading body: %w", err)
				}

				switch format {
				case "adf":
					bodyADF = content
				default: // markdown
					bodyADF, err = adf.FromMarkdown(content)
					if err != nil {
						return fmt.Errorf("converting markdown to ADF: %w", err)
					}
				}
			}

			updated, err := c.UpdateConfluencePage(pageID, pageTitle, bodyADF, current.Version+1)
			if err != nil {
				return fmt.Errorf("updating page: %w", err)
			}

			return OutputJSON(updated)
		},
	}

	cmd.Flags().StringVar(&title, "title", "", "new page title")
	cmd.Flags().StringVar(&body, "body", "", `new body content (use "-" for stdin, "@file" for file)`)
	cmd.Flags().StringVar(&format, "format", "markdown", "body format: markdown or adf")

	return cmd
}
