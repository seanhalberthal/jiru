package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/seanhalberthal/jiru/internal/client"
	"github.com/seanhalberthal/jiru/internal/config"
)

// cliClient is the shared client initialised by PersistentPreRunE.
var cliClient client.JiraClient

// cliConfig is the shared config loaded by PersistentPreRunE.
var cliConfig *config.Config

// InitClient loads config and creates a client for CLI subcommands.
func InitClient() error {
	return InitClientWithProfile("")
}

// InitClientWithProfile loads config for a specific profile and creates a client.
// Automatically migrates legacy config.env to profiles.yaml if needed.
func InitClientWithProfile(profile string) error {
	_ = config.MigrateToProfiles()
	cfg, err := config.LoadProfile(profile)
	if err != nil {
		return fmt.Errorf("configuration error: %w\nRun 'jiru' or 'jiru --reset' to configure", err)
	}
	cliConfig = cfg
	cliClient = client.New(cfg)
	return nil
}

// Client returns the shared CLI client.
func Client() client.JiraClient {
	return cliClient
}

// Config returns the shared CLI config.
func Config() *config.Config {
	return cliConfig
}

// OutputJSON writes v as indented JSON to stdout.
func OutputJSON(v any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

// OutputError writes a JSON error to stderr and exits.
func OutputError(err error) {
	fmt.Fprintf(os.Stderr, `{"error": %q}`+"\n", err.Error())
	os.Exit(1)
}
