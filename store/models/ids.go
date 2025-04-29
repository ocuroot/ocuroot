package models

import (
	"github.com/oklog/ulid/v2"
)

func NewID[T ~string]() T {
	return T(ulid.Make().String())
}

// ID type definitions
type RepoRemoteURL string
type ReleaseID string
type FunctionChainID string
type FunctionID string
type EnvironmentID string
type PhaseID string
type LogID string
