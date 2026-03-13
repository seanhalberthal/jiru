package validate

import "testing"

func TestIssueKey(t *testing.T) {
	tests := []struct {
		key   string
		valid bool
	}{
		{"PROJ-123", true},
		{"AB-1", true},
		{"LONGPROJ-99999", true},
		{"A-1", true},
		{"proj-123", false},                   // lowercase
		{"PROJ123", false},                    // missing dash
		{"PROJ-", false},                      // missing number
		{"-123", false},                       // missing project
		{"", false},                           // empty
		{"PROJ-0", true},                      // zero is valid
		{"PROJ-123 OR project = EVIL", false}, // injection attempt
		{"PROJ-123)", false},                  // JQL metacharacter
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			err := IssueKey(tt.key)
			if tt.valid && err != nil {
				t.Errorf("IssueKey(%q) returned error: %v", tt.key, err)
			}
			if !tt.valid && err == nil {
				t.Errorf("IssueKey(%q) expected error, got nil", tt.key)
			}
		})
	}
}

func TestProjectKey(t *testing.T) {
	tests := []struct {
		key   string
		valid bool
	}{
		{"PROJ", true},
		{"A", true},
		{"AB_CD", true},
		{"ABCDEFGHIJ", true},   // 10 chars (max)
		{"ABCDEFGHIJK", false}, // 11 chars (too long)
		{"proj", false},        // lowercase
		{"1PROJ", false},       // starts with digit
		{"", false},
		{"PROJ 123", false},               // space
		{"PROJ OR project = EVIL", false}, // injection
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			err := ProjectKey(tt.key)
			if tt.valid && err != nil {
				t.Errorf("ProjectKey(%q) returned error: %v", tt.key, err)
			}
			if !tt.valid && err == nil {
				t.Errorf("ProjectKey(%q) expected error, got nil", tt.key)
			}
		})
	}
}
