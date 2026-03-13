package validate

import (
	"fmt"
	"regexp"
)

var (
	issueKeyRe   = regexp.MustCompile(`^[A-Z][A-Z0-9]*-[0-9]+$`)
	projectKeyRe = regexp.MustCompile(`^[A-Z][A-Z0-9_]{0,9}$`)
)

// IssueKey validates a Jira issue key (e.g., "PROJ-123").
func IssueKey(key string) error {
	if !issueKeyRe.MatchString(key) {
		return fmt.Errorf("invalid issue key %q: must match [A-Z]+-[0-9]+", key)
	}
	return nil
}

// ProjectKey validates a Jira project key (e.g., "PROJ").
func ProjectKey(key string) error {
	if !projectKeyRe.MatchString(key) {
		return fmt.Errorf("invalid project key %q: must match [A-Z][A-Z0-9_]{0,9}", key)
	}
	return nil
}
