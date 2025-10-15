package models

import (
	"github.com/ocuroot/gittools"
	"github.com/ocuroot/ocuroot/refs"
	"github.com/ocuroot/ocuroot/sdk"
)

type RunType string

const (
	RunTypeUp   RunType = "up"
	RunTypeDown RunType = "down"
	RunTypeTask RunType = "task"
)

type Intent struct {
	Release refs.Ref                       `json:"release"`
	Inputs  map[string]sdk.InputDescriptor `json:"input"`
}

type Task struct {
	RunRef refs.Ref `json:"run_ref"`
	Type   RunType  `json:"type"`
	Intent
	Outputs map[string]any `json:"output"`
}

// Run represents a call or deploy
type Run struct {
	Type    RunType  `json:"type"`
	Release refs.Ref `json:"release"`

	Functions  []*Function    `json:"functions"`
	Outputs    map[string]any `json:"output"`
	WatchFiles []string       `json:"watch_files"`
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
	Remotes []gittools.Remote
	Source  []byte `json:"source"`
}

// PushIndex stores details about the most recent push
// This will be used to determine if releases are needed
type PushIndex struct {
	Commit         string `json:"commit"`
	PreviousCommit string `json:"previous_commit"`

	ReleaseConfigs map[string]ReleaseConfig `json:"release_configs"`
}

type ReleaseConfig struct {
	WatchFiles []string `json:"watch_files"`
}
