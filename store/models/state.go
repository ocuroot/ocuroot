package models

import (
	"github.com/ocuroot/ocuroot/refs"
	"github.com/ocuroot/ocuroot/sdk"
)

type WorkType string

const (
	WorkTypeUp   WorkType = "up"
	WorkTypeDown WorkType = "down"
	WorkTypeCall WorkType = "call"
)

type Intent struct {
	Type    WorkType                       `json:"type"`
	Release refs.Ref                       `json:"release"`
	Inputs  map[string]sdk.InputDescriptor `json:"input"`
}

// Work represents a call or deploy
type Work struct {
	Type    WorkType `json:"type"`
	Release refs.Ref `json:"release"`

	Functions []*Function    `json:"functions"`
	Outputs   map[string]any `json:"output"`
}

type Function struct {
	Fn           sdk.FunctionDef                `json:"fn"`
	Dependencies []refs.Ref                     `json:"dependencies,omitempty"`
	Inputs       map[string]sdk.InputDescriptor `json:"inputs"`
}

type Environment struct {
	Name       string         `json:"name"`
	Attributes map[string]any `json:"attributes"`
}

type RepoConfig struct {
	Source []byte `json:"source"`
}
