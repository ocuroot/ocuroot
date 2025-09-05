package models

import (
	"github.com/ocuroot/ocuroot/refs"
	"github.com/ocuroot/ocuroot/sdk"
)

type JobType string

const (
	JobTypeUp   JobType = "up"
	JobTypeDown JobType = "down"
	JobTypeTask JobType = "task"
)

type Intent struct {
	Type    JobType                        `json:"type"`
	Release refs.Ref                       `json:"release"`
	Inputs  map[string]sdk.InputDescriptor `json:"input"`
}

type Task struct {
	RunRef refs.Ref `json:"run_ref"`
	Intent
	Outputs map[string]any `json:"output"`
}

// Run represents a call or deploy
type Run struct {
	Type    JobType  `json:"type"`
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
