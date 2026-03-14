package config

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// zshExportRe matches lines like:
//
//	export KEY="value"
//	export KEY='value'
//	export KEY=value
//
// It captures the key and the raw value (with optional quotes).
var zshExportRe = regexp.MustCompile(
	`^\s*export\s+([A-Za-z_][A-Za-z0-9_]*)=(.*)`,
)

// relevantKeys is the set of environment variable names we care about.
// Includes aliases (JIRA_URL, JIRA_USERNAME) used by other tools like mcp-atlassian.
var relevantKeys = map[string]bool{
	"JIRA_DOMAIN":    true,
	"JIRA_URL":       true, // alias for JIRA_DOMAIN (full URL, protocol stripped)
	"JIRA_USER":      true,
	"JIRA_USERNAME":  true, // alias for JIRA_USER
	"JIRA_API_TOKEN": true,
	"JIRA_AUTH_TYPE": true,
	"JIRA_BOARD_ID":  true,
	"JIRA_PROJECT":   true,
}

// zshSearchPaths returns the ordered list of zsh files to scan.
func zshSearchPaths() []string {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}

	return []string{
		filepath.Join(home, ".zshenv"),
		filepath.Join(home, ".zprofile"),
		filepath.Join(home, ".zshrc"),
		filepath.Join(home, ".secrets.zsh"),
		filepath.Join(home, ".config", "secrets.zsh"),
		filepath.Join(home, ".config", "zsh", "secrets.zsh"),
	}
}

// parseZshExports scans the given file for export statements and returns
// a map of key=value pairs for the keys we care about.
func parseZshExports(path string) (map[string]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()

	result := make(map[string]string)
	scanner := bufio.NewScanner(f)

	for scanner.Scan() {
		line := scanner.Text()

		matches := zshExportRe.FindStringSubmatch(line)
		if matches == nil {
			continue
		}

		key := matches[1]
		if !relevantKeys[key] {
			continue
		}

		value := cleanExportValue(matches[2])
		if value != "" {
			result[key] = value
		}
	}

	return result, scanner.Err()
}

// cleanExportValue strips surrounding quotes and inline comments from
// a raw export value.
func cleanExportValue(raw string) string {
	raw = strings.TrimSpace(raw)

	// Handle double-quoted values.
	if len(raw) >= 2 && raw[0] == '"' {
		if end := strings.Index(raw[1:], "\""); end >= 0 {
			return raw[1 : end+1]
		}
	}

	// Handle single-quoted values.
	if len(raw) >= 2 && raw[0] == '\'' {
		if end := strings.Index(raw[1:], "'"); end >= 0 {
			return raw[1 : end+1]
		}
	}

	// Unquoted — strip trailing comment.
	if idx := strings.Index(raw, " #"); idx >= 0 {
		raw = raw[:idx]
	}
	if idx := strings.Index(raw, "\t#"); idx >= 0 {
		raw = raw[:idx]
	}

	return strings.TrimSpace(raw)
}

// loadZshCredentials scans zsh config files and returns a map of
// JIRA_* key=value pairs found. First value wins per key.
func loadZshCredentials() map[string]string {
	merged := make(map[string]string)

	for _, path := range zshSearchPaths() {
		exports, err := parseZshExports(path)
		if err != nil {
			continue // file doesn't exist or isn't readable — skip
		}

		for k, v := range exports {
			if _, exists := merged[k]; !exists {
				merged[k] = v
			}
		}
	}

	return merged
}
