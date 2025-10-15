package refstore

import (
	"context"
	"errors"
)

var ErrReadOnly = errors.New("read-only store")

type ReadOnlyStore struct {
	store Store
}

func NewReadOnlyStore(store Store) Store {
	return &ReadOnlyStore{store: store}
}

func (r *ReadOnlyStore) Info() StoreInfo {
	return r.store.Info()
}

// AddDependency implements RefStore.
func (r *ReadOnlyStore) AddDependency(ctx context.Context, ref string, dependency string) error {
	return ErrReadOnly
}

// Close implements RefStore.
func (r *ReadOnlyStore) Close() error {
	return r.store.Close()
}

// CommitTransaction implements RefStore.
func (r *ReadOnlyStore) CommitTransaction(ctx context.Context) error {
	return ErrReadOnly
}

// Delete implements RefStore.
func (r *ReadOnlyStore) Delete(ctx context.Context, ref string) error {
	return ErrReadOnly
}

// Get implements RefStore.
func (r *ReadOnlyStore) Get(ctx context.Context, ref string, v any) error {
	return r.store.Get(ctx, ref, v)
}

// GetDependants implements RefStore.
func (r *ReadOnlyStore) GetDependants(ctx context.Context, ref string) ([]string, error) {
	return r.store.GetDependants(ctx, ref)
}

// GetDependencies implements RefStore.
func (r *ReadOnlyStore) GetDependencies(ctx context.Context, ref string) ([]string, error) {
	return r.store.GetDependencies(ctx, ref)
}

// GetLinks implements RefStore.
func (r *ReadOnlyStore) GetLinks(ctx context.Context, ref string) ([]string, error) {
	return r.store.GetLinks(ctx, ref)
}

// Link implements RefStore.
func (r *ReadOnlyStore) Link(ctx context.Context, ref string, target string) error {
	return ErrReadOnly
}

// Unlink implements RefStore.
func (r *ReadOnlyStore) Unlink(ctx context.Context, ref string) error {
	return ErrReadOnly
}

// Match implements RefStore.
func (r *ReadOnlyStore) Match(ctx context.Context, glob ...string) ([]string, error) {
	return r.store.Match(ctx, glob...)
}

// MatchOptions implements RefStore.
func (r *ReadOnlyStore) MatchOptions(ctx context.Context, options MatchOptions, glob ...string) ([]string, error) {
	return r.store.MatchOptions(ctx, options, glob...)
}

// RemoveDependency implements RefStore.
func (r *ReadOnlyStore) RemoveDependency(ctx context.Context, ref string, dependency string) error {
	return ErrReadOnly
}

// ResolveLink implements RefStore.
func (r *ReadOnlyStore) ResolveLink(ctx context.Context, ref string) (string, error) {
	return r.store.ResolveLink(ctx, ref)
}

// Set implements RefStore.
func (r *ReadOnlyStore) Set(ctx context.Context, ref string, v any) error {
	return ErrReadOnly
}

// StartTransaction implements RefStore.
func (r *ReadOnlyStore) StartTransaction(ctx context.Context, message string) error {
	return ErrReadOnly
}
