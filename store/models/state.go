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
	// Entrypoint is the ref of the first function in the chain
	Entrypoint refs.Ref       `json:"entrypoint"`
	Outputs    map[string]any `json:"output"`
}

type Function struct {
	ID           FunctionID                     `json:"id"`
	Fn           sdk.FunctionDef                `json:"fn"`
	Status       Status                         `json:"status"`
	Dependencies []refs.Ref                     `json:"dependencies,omitempty"`
	Inputs       map[string]sdk.InputDescriptor `json:"inputs"`
	Outputs      map[string]any                 `json:"outputs,omitempty"`
}
