package work

import "github.com/ocuroot/ocuroot/refs"

type Work struct {
	Ref      refs.Ref `json:"ref"`
	WorkType WorkType `json:"work_type"`
	Commit   string   `json:"commit,omitempty"`

	Children []Work `json:"children,omitempty"`
}

type WorkType string

const (
	// WorkTypeCreate is work to create a state doc from a new intent doc
	WorkTypeCreate WorkType = "create"
	// WorkTypeDelete is work to delete a state doc because the corresponding intent doc does not exist
	WorkTypeDelete WorkType = "delete"
	// WorkTypeUpdate is work to update a state doc because it differs from the corresponding intent doc
	WorkTypeUpdate WorkType = "update"
	// WorkTypeRun is work to run a task, like a build or deploy
	WorkTypeRun WorkType = "run"
	// WorkTypeRelease is work to begin or continue a release at the current commit
	WorkTypeRelease WorkType = "release"
	// WorkTypeOps is work to perform an operation on an existing release
	WorkTypeOp WorkType = "op"
)
