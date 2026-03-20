package cli

import (
	"github.com/spf13/cobra"
)

// WikiCmd returns the 'wiki' command group for Confluence operations.
func WikiCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "wiki",
		Short: "Confluence wiki commands",
	}

	cmd.AddCommand(wikiSpacesCmd(), wikiPageCmd())
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
	return &cobra.Command{
		Use:   "page <page-id>",
		Short: "Fetch a Confluence page as JSON",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			page, err := Client().ConfluencePage(args[0])
			if err != nil {
				return err
			}
			return OutputJSON(page)
		},
	}
}
