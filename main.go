package main

import (
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
	if len(os.Args) > 1 && os.Args[1] == "--version" {
		fmt.Println("jiru", version)
		return
	}

	var directIssue string
	if len(os.Args) > 1 && os.Args[1] != "--version" {
		directIssue = os.Args[1]
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
