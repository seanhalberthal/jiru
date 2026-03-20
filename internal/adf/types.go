package adf

// Document is the root ADF node.
type Document struct {
	Type    string `json:"type"`    // always "doc"
	Version int    `json:"version"` // typically 1
	Content []Node `json:"content"`
}

// Node is a generic ADF node (block or inline).
type Node struct {
	Type    string         `json:"type"`
	Content []Node         `json:"content,omitempty"`
	Text    string         `json:"text,omitempty"`
	Marks   []Mark         `json:"marks,omitempty"`
	Attrs   map[string]any `json:"attrs,omitempty"`
}

// Mark is a text formatting annotation.
type Mark struct {
	Type  string         `json:"type"`
	Attrs map[string]any `json:"attrs,omitempty"`
}
