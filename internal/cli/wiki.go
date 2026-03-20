package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/seanhalberthal/jiru/internal/adf"
)

// WikiCmd returns the 'wiki' command group for Confluence operations.
func WikiCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "wiki",
		Short: "Confluence wiki commands",
	}

	cmd.AddCommand(wikiSpacesCmd(), wikiPageCmd(), wikiPagesCmd(), wikiSearchCmd(), wikiEditCmd())
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

func wikiPagesCmd() *cobra.Command {
	var limit int

	cmd := &cobra.Command{
		Use:   "pages <space-id>",
		Short: "List pages in a Confluence space",
		Long: `List pages in a Confluence space by its numeric space ID.

Use 'jiru wiki spaces' to list spaces and find the numeric ID in the "id" field.`,
		Args: cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			pages, err := Client().ConfluenceSpacePages(args[0], limit)
			if err != nil {
				return err
			}
			// Output compact summary for easy scanning.
			type row struct {
				ID    string `json:"id"`
				Title string `json:"title"`
			}
			rows := make([]row, 0, len(pages))
			for _, p := range pages {
				rows = append(rows, row{ID: p.ID, Title: p.Title})
			}
			return OutputJSON(rows)
		},
	}

	cmd.Flags().IntVar(&limit, "limit", 50, "maximum number of pages to return")
	return cmd
}

func wikiSearchCmd() *cobra.Command {
	var limit int

	cmd := &cobra.Command{
		Use:   "search <cql>",
		Short: "Search Confluence using CQL",
		Long: `Search Confluence pages using CQL (Confluence Query Language).

Examples:
  jiru wiki search 'space = "~username" AND title ~ "meeting"'
  jiru wiki search 'type = page AND text ~ "deployment"'`,
		Args: cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			results, err := Client().ConfluenceSearchCQL(args[0], limit)
			if err != nil {
				return err
			}
			return OutputJSON(results)
		},
	}

	cmd.Flags().IntVar(&limit, "limit", 25, "maximum number of results to return")
	return cmd
}

func wikiPageCmd() *cobra.Command {
	var format string

	cmd := &cobra.Command{
		Use:   "page <page-id>",
		Short: "Fetch a Confluence page",
		Long: `Fetch a Confluence page by its numeric page ID.

Use 'jiru wiki spaces' to list spaces, then 'jiru wiki pages <space-id>' to
find page IDs.

Output format is controlled by --format:
  json       Full page as JSON (default)
  markdown   ADF body converted to Markdown
  adf        Raw ADF JSON body only`,
		Args: cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			page, err := Client().ConfluencePage(args[0])
			if err != nil {
				fmt.Fprintf(os.Stderr, "\nHint: make sure you provide a page ID, not a space ID.\n"+
					"Run 'jiru wiki spaces' then 'jiru wiki pages <space-id>' to discover page IDs.\n\n")
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
