package work

import "github.com/ocuroot/ocuroot/refs"

type Work struct {
	Ref      refs.Ref `json:"ref"`
	WorkType WorkType `json:"work_type"`

	Children []Work `json:"children,omitempty"`
}

type WorkType string

const (
	WorkTypeCreate WorkType = "create"
	WorkTypeDelete WorkType = "delete"
	WorkTypeUpdate WorkType = "update"
	WorkTypeRun    WorkType = "run"
	WorkTypeOp     WorkType = "op"
)
