package refstore

import (
	"context"
	"encoding/json"
	"errors"
)

var ErrRefNotFound = errors.New("ref not found")

type Store interface {
	Info() StoreInfo

	StartTransaction(ctx context.Context, message string) error
	CommitTransaction(ctx context.Context) error

	Get(ctx context.Context, ref string, v any) error
	Set(ctx context.Context, ref string, v any) error
	Delete(ctx context.Context, ref string) error

	// Match will return all the refs matching the provided glob patterns
	// See: https://en.wikipedia.org/wiki/Glob_(programming)
	Match(ctx context.Context, glob ...string) ([]string, error)
	MatchOptions(ctx context.Context, options MatchOptions, glob ...string) ([]string, error)

	// Link will create a new ref that points to the target ref
	Link(ctx context.Context, ref string, target string) error
	Unlink(ctx context.Context, ref string) error
	GetLinks(ctx context.Context, ref string) ([]string, error)
	ResolveLink(ctx context.Context, ref string) (string, error)

	Close() error
}

type StoreInfo struct {
	Version int `json:"version"`

	Tags map[string]struct{} `json:"tags"`
}

type MatchOptions struct {
	NoLinks bool
}

type StorageKind string

const (
	StorageKindRef  StorageKind = "ref"
	StorageKindLink StorageKind = "link"
)

type StorageObject struct {
	Kind        StorageKind     `json:"kind"`
	BodyType    string          `json:"body_type,omitempty"`
	CreateStack []string        `json:"create_stack,omitempty"`
	SetStack    []string        `json:"set_stack,omitempty"`
	Links       []string        `json:"links,omitempty"`
	Body        json.RawMessage `json:"body"`
}
