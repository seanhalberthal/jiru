# JTUI-002: Load Credentials from Zsh Configuration Files

## Overview

Add a credential loading fallback that parses local zsh configuration files (`~/.zshrc`, `~/.zshenv`, `~/.zprofile`, custom secrets files) for `export JIRA_*=...` declarations. This sits between the existing env var check and the jira-cli config fallback, allowing users who define their Jira credentials in shell files to use the TUI without manually exporting them first.

## Prerequisites

- No new external dependencies required — this is pure file parsing with the standard library
- Understanding of common zsh file conventions and `export` syntax

## Why This Is Useful

Users commonly store secrets in files like:
- `~/.zshrc` — standard zsh config
- `~/.zshenv` — loaded for every zsh session (including non-interactive)
- `~/.zprofile` — login shell config
- `~/.secrets.zsh` / `~/.config/secrets.zsh` — custom secrets files sourced from `.zshrc`

If a user launches the TUI from a context where these haven't been sourced (e.g. a cron job, a different shell, an IDE terminal), the env vars won't exist. Parsing the files directly solves this.

## Design Decisions

### Parsing Approach

Parse `export` statements with a regular expression. We need to handle these common patterns:

```zsh
export JIRA_API_TOKEN="my-token"
export JIRA_API_TOKEN='my-token'
export JIRA_API_TOKEN=my-token
export JIRA_DOMAIN="foo.atlassian.net"   # with trailing comment
export JIRA_URL="https://foo.atlassian.net"  # alias for JIRA_DOMAIN (protocol stripped)
export JIRA_USERNAME="user@example.com"  # alias for JIRA_USER
```

**Aliases supported** (for compatibility with mcp-atlassian and similar tools):
- `JIRA_URL` → `JIRA_DOMAIN` (protocol prefix stripped automatically)
- `JIRA_USERNAME` → `JIRA_USER`

Canonical names (`JIRA_DOMAIN`, `JIRA_USER`) take precedence when both are set.

We do **not** need to handle:
- Variable interpolation (`export FOO=$BAR`) — too complex, not reliable without a full shell evaluator
- Multi-line values — not typical for these credential vars
- `source`/`.` directives to follow includes — out of scope for v1 (but see Future Work below)

### File Search Order

1. `~/.zshenv` (loaded first by zsh, most likely place for exports that should always exist)
2. `~/.zprofile`
3. `~/.zshrc`
4. `~/.secrets.zsh`
5. `~/.config/secrets.zsh`
6. `~/.config/zsh/secrets.zsh`

Files are read in order. **First value wins** — if `JIRA_API_TOKEN` is found in `~/.zshenv`, we don't overwrite it from `~/.zshrc`. This mirrors zsh's own loading order where `.zshenv` runs first.

### Config Loading Priority (Updated)

1. **Environment variables** (highest — already running shell)
2. **Zsh config files** (new — parsed from disk)
3. **jira-cli config** (`~/.config/.jira/.config.yml`)
4. **Validation error** (if still missing)

## File Structure

```
internal/config/
├── config.go      (modified — add zsh loading step)
└── zshparse.go    (new — zsh file parser)
```

## Implementation Steps

### 1. Create the Zsh Parser

**File**: `internal/config/zshparse.go`

```go
package config

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// zshExportRe matches lines like:
//   export KEY="value"
//   export KEY='value'
//   export KEY=value
// It captures the key and the raw value (with optional quotes).
var zshExportRe = regexp.MustCompile(
	`^\s*export\s+([A-Za-z_][A-Za-z0-9_]*)=(.*)`,
)

// relevantKeys is the set of environment variable names we care about.
// Includes aliases (JIRA_URL, JIRA_USERNAME) used by other tools like mcp-atlassian.
var relevantKeys = map[string]bool{
	"JIRA_DOMAIN":   true,
	"JIRA_URL":      true, // alias for JIRA_DOMAIN (full URL, protocol stripped)
	"JIRA_USER":     true,
	"JIRA_USERNAME": true, // alias for JIRA_USER
	"JIRA_API_TOKEN": true,
	"JIRA_AUTH_TYPE": true,
	"JIRA_BOARD_ID":  true,
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
	defer f.Close()

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
```

**Notes**:
- The regex intentionally avoids matching `=` inside values by capturing everything after the first `=` and post-processing
- `cleanExportValue` handles the three common quoting styles
- Files that don't exist are silently skipped — no errors propagated
- Variable interpolation (e.g. `$HOME`) is not resolved — values containing `$` will be taken literally, which is acceptable since credential values are typically literal strings

### 2. Integrate into Config Loading

**File**: `internal/config/config.go`

Modify the `Load()` function to call `loadZshCredentials()` after env vars but before jira-cli config:

```go
func Load() (*Config, error) {
	cfg := &Config{
		AuthType: "basic",
	}

	// 1. Environment variables take priority.
	cfg.Domain = os.Getenv("JIRA_DOMAIN")
	if cfg.Domain == "" {
		if u := os.Getenv("JIRA_URL"); u != "" {
			cfg.Domain = stripProtocol(u)
		}
	}
	cfg.User = os.Getenv("JIRA_USER")
	if cfg.User == "" {
		cfg.User = os.Getenv("JIRA_USERNAME")
	}
	cfg.APIToken = os.Getenv("JIRA_API_TOKEN")

	if at := os.Getenv("JIRA_AUTH_TYPE"); at != "" {
		cfg.AuthType = at
	}

	if bid := os.Getenv("JIRA_BOARD_ID"); bid != "" {
		id, err := strconv.Atoi(bid)
		if err != nil {
			return nil, fmt.Errorf("invalid JIRA_BOARD_ID: %w", err)
		}
		cfg.BoardID = id
	}

	// 2. Fill gaps from zsh config files (e.g. ~/.zshrc, ~/.secrets.zsh).
	if cfg.Domain == "" || cfg.User == "" || cfg.APIToken == "" || cfg.BoardID == 0 {
		cfg.applyZshCredentials()
	}

	// 3. Fall back to jira-cli config for missing values.
	if cfg.Domain == "" || cfg.User == "" || cfg.BoardID == 0 {
		if err := cfg.loadJiraCliConfig(); err == nil {
			// Only fill in what's missing.
		}
	}

	// Validate required fields.
	if cfg.Domain == "" {
		return nil, fmt.Errorf("JIRA_DOMAIN or JIRA_URL is required (set env var, add to ~/.zshrc, or configure jira-cli)")
	}
	if cfg.User == "" {
		return nil, fmt.Errorf("JIRA_USER or JIRA_USERNAME is required (set env var, add to ~/.zshrc, or configure jira-cli)")
	}
	if cfg.APIToken == "" {
		return nil, fmt.Errorf("JIRA_API_TOKEN is required (set env var or add to ~/.zshrc / ~/.secrets.zsh)")
	}
	if cfg.BoardID == 0 {
		return nil, fmt.Errorf("JIRA_BOARD_ID is required (set env var, add to ~/.zshrc, or configure jira-cli)")
	}

	return cfg, nil
}

// applyZshCredentials fills missing config values from zsh config files.
// Supports aliases: JIRA_URL → Domain (with protocol stripping), JIRA_USERNAME → User.
func (c *Config) applyZshCredentials() {
	creds := loadZshCredentials()

	if c.Domain == "" {
		if d := creds["JIRA_DOMAIN"]; d != "" {
			c.Domain = d
		} else if u := creds["JIRA_URL"]; u != "" {
			c.Domain = stripProtocol(u)
		}
	}
	if c.User == "" {
		if u := creds["JIRA_USER"]; u != "" {
			c.User = u
		} else if u := creds["JIRA_USERNAME"]; u != "" {
			c.User = u
		}
	}
	if c.APIToken == "" {
		c.APIToken = creds["JIRA_API_TOKEN"]
	}
	if c.AuthType == "basic" {
		if at, ok := creds["JIRA_AUTH_TYPE"]; ok {
			c.AuthType = at
		}
	}
	if c.BoardID == 0 {
		if bid, ok := creds["JIRA_BOARD_ID"]; ok {
			if id, err := strconv.Atoi(bid); err == nil {
				c.BoardID = id
			}
		}
	}
}

// stripProtocol removes http:// or https:// prefix from a URL.
func stripProtocol(url string) string {
	for _, prefix := range []string{"https://", "http://"} {
		if len(url) > len(prefix) && url[:len(prefix)] == prefix {
			return url[len(prefix):]
		}
	}
	return url
}
```

### 3. Add Tests for the Zsh Parser

**File**: `internal/config/zshparse_test.go`

```go
package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCleanExportValue(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{"double quoted", `"my-token"`, "my-token"},
		{"single quoted", `'my-token'`, "my-token"},
		{"unquoted", `my-token`, "my-token"},
		{"double quoted with comment", `"my-token" # jira token`, "my-token"},
		{"unquoted with comment", `my-token # jira token`, "my-token"},
		{"empty", ``, ""},
		{"spaces", `  my-token  `, "my-token"},
		{"quoted empty", `""`, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := cleanExportValue(tt.raw)
			if got != tt.want {
				t.Errorf("cleanExportValue(%q) = %q, want %q", tt.raw, got, tt.want)
			}
		})
	}
}

func TestParseZshExports(t *testing.T) {
	content := `# Jira config
export JIRA_DOMAIN="myco.atlassian.net"
export JIRA_USER='user@example.com'
export JIRA_API_TOKEN=abc123
export JIRA_BOARD_ID=42
export JIRA_AUTH_TYPE="bearer"
export JIRA_URL="https://alt.atlassian.net"
export JIRA_USERNAME="alt@example.com"
export UNRELATED_VAR="ignored"

# This line is just a comment
some_command --flag
`

	dir := t.TempDir()
	path := filepath.Join(dir, ".zshrc")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	got, err := parseZshExports(path)
	if err != nil {
		t.Fatal(err)
	}

	expected := map[string]string{
		"JIRA_DOMAIN":    "myco.atlassian.net",
		"JIRA_USER":      "user@example.com",
		"JIRA_API_TOKEN": "abc123",
		"JIRA_BOARD_ID":  "42",
		"JIRA_AUTH_TYPE": "bearer",
		"JIRA_URL":       "https://alt.atlassian.net",
		"JIRA_USERNAME":  "alt@example.com",
	}

	for k, want := range expected {
		if got[k] != want {
			t.Errorf("key %s: got %q, want %q", k, got[k], want)
		}
	}

	if _, exists := got["UNRELATED_VAR"]; exists {
		t.Error("should not capture UNRELATED_VAR")
	}
}

func TestStripProtocol(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"https://myco.atlassian.net", "myco.atlassian.net"},
		{"http://myco.atlassian.net", "myco.atlassian.net"},
		{"myco.atlassian.net", "myco.atlassian.net"},
	}
	for _, tt := range tests {
		got := stripProtocol(tt.input)
		if got != tt.want {
			t.Errorf("stripProtocol(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestParseZshExports_MissingFile(t *testing.T) {
	_, err := parseZshExports("/nonexistent/path/.zshrc")
	if err == nil {
		t.Error("expected error for missing file")
	}
}

func TestLoadZshCredentials_FirstValueWins(t *testing.T) {
	dir := t.TempDir()

	// Simulate two files where the first has a value and the second has a different one.
	file1 := filepath.Join(dir, "first.zsh")
	file2 := filepath.Join(dir, "second.zsh")

	os.WriteFile(file1, []byte(`export JIRA_API_TOKEN="first-token"`+"\n"), 0644)
	os.WriteFile(file2, []byte(`export JIRA_API_TOKEN="second-token"`+"\n"), 0644)

	// Parse individually and verify first-wins behaviour.
	exports1, _ := parseZshExports(file1)
	exports2, _ := parseZshExports(file2)

	merged := make(map[string]string)
	for k, v := range exports1 {
		if _, exists := merged[k]; !exists {
			merged[k] = v
		}
	}
	for k, v := range exports2 {
		if _, exists := merged[k]; !exists {
			merged[k] = v
		}
	}

	if merged["JIRA_API_TOKEN"] != "first-token" {
		t.Errorf("expected first-token, got %s", merged["JIRA_API_TOKEN"])
	}
}
```

## Critical Implementation Details

### Security Considerations

- We only read files owned by the current user in their home directory — no privilege escalation risk
- The parser ignores variable interpolation (`$VAR`), so it won't accidentally resolve or leak unrelated variables
- Credential values are held in memory only (same as the existing env var approach)

### Edge Cases

- **File doesn't exist**: Silently skipped — no error
- **File not readable** (permissions): Silently skipped
- **Empty value**: `export JIRA_API_TOKEN=""` results in empty string, which is treated as "not set"
- **Duplicate exports in same file**: Last one in the file wins (within that file)
- **Commented-out exports**: Lines starting with `#` won't match the regex (leading `#` prevents the `export` keyword match)
- **Inline `export` in conditionals**: e.g. `if ...; export FOO=bar` — would match. This is an acceptable edge case since it's rare and the user likely intends the value

### Regex Breakdown

```
^\s*export\s+([A-Za-z_][A-Za-z0-9_]*)=(.*)
```

- `^\s*` — optional leading whitespace
- `export\s+` — the keyword followed by whitespace
- `([A-Za-z_][A-Za-z0-9_]*)` — captures the variable name (valid shell identifier)
- `=` — literal equals
- `(.*)` — captures everything after `=` (cleaned up by `cleanExportValue`)

## Testing

### Test Scenarios
- ✅ Double-quoted values parsed correctly
- ✅ Single-quoted values parsed correctly
- ✅ Unquoted values parsed correctly
- ✅ Inline comments stripped
- ✅ Non-JIRA variables ignored
- ✅ Missing files silently skipped
- ✅ First-value-wins behaviour across multiple files
- ✅ Empty values treated as unset
- ✅ `JIRA_URL` alias: protocol stripped, maps to Domain
- ✅ `JIRA_USERNAME` alias: maps to User
- ✅ Canonical names (`JIRA_DOMAIN`, `JIRA_USER`) take precedence over aliases
- ✅ Integration: env vars still take priority over zsh-parsed values
- ✅ Integration: zsh-parsed values take priority over jira-cli config

### Test File Location
**File**: `internal/config/zshparse_test.go`

### Running Tests
```bash
go test ./internal/config/ -v -run TestCleanExportValue
go test ./internal/config/ -v -run TestParseZshExports
go test ./internal/config/ -v
```

## Future Work (Out of Scope)

- **Follow `source` directives**: Parse `source ~/.secrets.zsh` from `.zshrc` and follow the chain. Would make it more robust but adds complexity.
- **Configurable search paths**: Let users specify additional files via a `--zsh-files` flag or config option.
- **Bash support**: Same approach could work for `~/.bashrc`, `~/.bash_profile` etc. The regex is compatible.

## Key Files Referenced

- `internal/config/config.go` — Existing config loading logic, modified to add zsh fallback step
- `internal/config/zshparse.go` — New file: zsh file parser and credential extractor
- `internal/config/zshparse_test.go` — New file: tests for the parser
- `main.go` — Entry point, calls `config.Load()` (no changes needed)
