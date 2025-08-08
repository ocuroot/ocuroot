package refstore

import (
	"context"
	"errors"
)

var ErrRefNotFound = errors.New("ref not found")

type Store interface {
	StartTransaction(ctx context.Context) error
	CommitTransaction(ctx context.Context, message string) error

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

	// Dependencies track dependency relationships between refs
	AddDependency(ctx context.Context, ref string, dependency string) error
	RemoveDependency(ctx context.Context, ref string, dependency string) error
	GetDependencies(ctx context.Context, ref string) ([]string, error)
	GetDependants(ctx context.Context, ref string) ([]string, error)

	Close() error
}

type MatchOptions struct {
	NoLinks bool
}
