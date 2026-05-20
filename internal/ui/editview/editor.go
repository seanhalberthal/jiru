package editview

import (
	"os"
	"os/exec"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// editorReturnedMsg is dispatched after the external editor exits. When ok is
// true, content holds the new description body; on any failure the message is
// delivered with ok=false so Update can simply do nothing.
type editorReturnedMsg struct {
	content string
	ok      bool
}

// openInEditor writes the current description to a temp file and shells out to
// $VISUAL / $EDITOR (falling back to nvim/vim/vi). tea.ExecProcess suspends the
// TUI while the editor runs and restores it on exit.
func openInEditor(initial string) tea.Cmd {
	f, err := os.CreateTemp("", "jiru-desc-*.md")
	if err != nil {
		return func() tea.Msg { return editorReturnedMsg{} }
	}
	path := f.Name()
	if _, err := f.WriteString(initial); err != nil {
		_ = f.Close()
		_ = os.Remove(path)
		return func() tea.Msg { return editorReturnedMsg{} }
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(path)
		return func() tea.Msg { return editorReturnedMsg{} }
	}

	parts := strings.Fields(resolveEditor())
	if len(parts) == 0 {
		_ = os.Remove(path)
		return func() tea.Msg { return editorReturnedMsg{} }
	}
	args := append(parts[1:], path)
	cmd := exec.Command(parts[0], args...)

	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		defer func() { _ = os.Remove(path) }()
		if err != nil {
			return editorReturnedMsg{}
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return editorReturnedMsg{}
		}
		return editorReturnedMsg{content: string(data), ok: true}
	})
}

func resolveEditor() string {
	if v := strings.TrimSpace(os.Getenv("VISUAL")); v != "" {
		return v
	}
	if v := strings.TrimSpace(os.Getenv("EDITOR")); v != "" {
		return v
	}
	for _, cand := range []string{"nvim", "vim", "vi"} {
		if _, err := exec.LookPath(cand); err == nil {
			return cand
		}
	}
	return "vi"
}
