package mermaid

import "strings"

// EscapeMermaidNodeId escapes node IDs that contain spaces or other special characters
func EscapeMermaidNodeId(id string) string {
	return strings.ReplaceAll(id, " ", "_")
}
