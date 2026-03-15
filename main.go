package main

import (
	"flag"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/seanhalberthal/jiru/internal/client"
	"github.com/seanhalberthal/jiru/internal/config"
	"github.com/seanhalberthal/jiru/internal/ui"
	"github.com/seanhalberthal/jiru/internal/validate"
)

var version = "dev"

func main() {
	showVersion := flag.Bool("version", false, "print version and exit")
	flag.BoolVar(showVersion, "v", false, "print version and exit")

	reset := flag.Bool("reset", false, "reset all config and credentials, then start the setup wizard")
	flag.BoolVar(reset, "r", false, "reset all config and credentials, then start the setup wizard")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, `Usage: jiru [flags] [ISSUE-KEY]

A terminal UI for Jira.

Flags:
  -h, --help       show this help message
  -v, --version    print version and exit
  -r, --reset      reset all config and credentials, then start the setup wizard
`)
	}
	flag.Parse()

	if *showVersion {
		fmt.Println("jiru", version)
		return
	}

	if *reset {
		fmt.Print("This will remove all config and credentials. Continue? [y/N] ")
		var answer string
		_, _ = fmt.Scanln(&answer)
		if answer != "y" && answer != "Y" {
			fmt.Println("Cancelled.")
			return
		}
		if err := config.ResetConfig(); err != nil {
			fmt.Fprintf(os.Stderr, "Error resetting config: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("Config and credentials cleared. Starting setup wizard...")

		// Launch directly into the setup wizard with an empty config.
		empty := &config.Config{AuthType: "basic"}
		allMissing := []string{"domain", "user", "api_token"}
		p := tea.NewProgram(
			ui.NewApp(nil, "", empty, allMissing),
			tea.WithAltScreen(),
		)
		if _, err := p.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	var directIssue string
	if flag.NArg() > 0 {
		directIssue = flag.Arg(0)
		if err := validate.IssueKey(directIssue); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	}

	partial, missing := config.PartialLoad()

	var c client.JiraClient
	if len(missing) == 0 {
		cfg, err := config.Load()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Configuration error: %v\n", err)
			os.Exit(1)
		}
		c = client.New(cfg)
	}

	p := tea.NewProgram(
		ui.NewApp(c, directIssue, partial, missing),
		tea.WithAltScreen(),
	)

	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
