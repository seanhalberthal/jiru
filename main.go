package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"

	"github.com/seanhalberthal/jiru/internal/cli"
	"github.com/seanhalberthal/jiru/internal/client"
	"github.com/seanhalberthal/jiru/internal/config"
	"github.com/seanhalberthal/jiru/internal/filters"
	"github.com/seanhalberthal/jiru/internal/recents"
	"github.com/seanhalberthal/jiru/internal/ui"
	"github.com/seanhalberthal/jiru/internal/validate"
)

var version = "dev"

// profileFlag is the global --profile flag value.
var profileFlag string

func main() {
	var reset bool

	rootCmd := &cobra.Command{
		Use:     "jiru [issue-key]",
		Short:   "A terminal UI for Jira",
		Version: version,
		// Silence cobra's default error/usage printing — we handle it ourselves.
		SilenceErrors: true,
		SilenceUsage:  true,
		Args:          cobra.MaximumNArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			if reset {
				return runReset()
			}
			var directIssue string
			if len(args) > 0 {
				directIssue = args[0]
				if err := validate.IssueKey(directIssue); err != nil {
					return err
				}
			}
			return runTUI(directIssue)
		},
	}

	rootCmd.Flags().BoolVarP(&reset, "reset", "r", false, "reset all config and credentials, then start the setup wizard")
	rootCmd.PersistentFlags().StringVarP(&profileFlag, "profile", "p", "", "use a named profile")

	// CLI subcommands share a PersistentPreRunE for config/client init.
	cliGroup := &cobra.Group{ID: "cli", Title: "CLI Commands:"}
	rootCmd.AddGroup(cliGroup)

	getCmd := cli.GetCmd()
	searchCmd := cli.SearchCmd()
	listCmd := cli.ListCmd()
	boardsCmd := cli.BoardsCmd()
	wikiCmd := cli.WikiCmd()
	editCmd := cli.EditCmd()
	commentCmd := cli.CommentCmd()
	transitionCmd := cli.TransitionCmd()

	for _, cmd := range []*cobra.Command{getCmd, searchCmd, listCmd, boardsCmd, wikiCmd, editCmd, commentCmd, transitionCmd} {
		cmd.GroupID = "cli"
		cmd.PersistentPreRunE = func(_ *cobra.Command, _ []string) error {
			return cli.InitClientWithProfile(profileFlag)
		}
	}

	rootCmd.AddCommand(getCmd, searchCmd, listCmd, boardsCmd, wikiCmd, editCmd, commentCmd, transitionCmd)

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func runReset() error {
	fmt.Print("This will remove all config and credentials. Continue? [y/N] ")
	var answer string
	_, _ = fmt.Scanln(&answer)
	if answer != "y" && answer != "Y" {
		fmt.Println("Cancelled.")
		return nil
	}
	if err := config.ResetConfig(); err != nil {
		return fmt.Errorf("resetting config: %w", err)
	}
	fmt.Println("Config and credentials cleared. Starting setup wizard...")

	empty := &config.Config{AuthType: "basic"}
	allMissing := []string{"domain", "user", "api_token"}
	p := tea.NewProgram(
		ui.NewApp(nil, "", empty, allMissing, version),
		tea.WithAltScreen(),
	)
	_, err := p.Run()
	return err
}

func runTUI(directIssue string) error {
	// Set the filter and recents profile if specified.
	if profileFlag != "" {
		filters.SetProfile(profileFlag)
		recents.SetProfile(profileFlag)
	} else {
		filters.SetProfile(config.ActiveProfileName())
		recents.SetProfile(config.ActiveProfileName())
	}

	partial, missing := config.PartialLoadProfile(profileFlag)

	var c client.JiraClient
	if len(missing) == 0 {
		cfg, err := config.LoadProfile(profileFlag)
		if err != nil {
			return fmt.Errorf("configuration error: %w", err)
		}
		c = client.New(cfg)
	}

	app := ui.NewApp(c, directIssue, partial, missing, version)
	if profileFlag != "" {
		app.SetProfileName(profileFlag)
	} else {
		app.SetProfileName(config.ActiveProfileName())
	}

	p := tea.NewProgram(app, tea.WithAltScreen())
	_, err := p.Run()
	return err
}
