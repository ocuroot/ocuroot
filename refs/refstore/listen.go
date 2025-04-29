package refstore

import (
	"context"

	"github.com/gobwas/glob"
)

func ListenToStateChanges(f func(ctx context.Context, ref string), store Store, filters ...string) (Store, error) {
	globs := make([]glob.Glob, 0)
	for _, filter := range filters {
		glob, err := glob.Compile(filter)
		if err != nil {
			return nil, err
		}
		globs = append(globs, glob)
	}
	return &stateListener{
		store:    store,
		filters:  globs,
		callback: f,
	}, nil
}

var _ Store = &stateListener{}

type stateListener struct {
	store    Store
	filters  []glob.Glob
	callback func(ctx context.Context, ref string)

	inTransaction   bool
	transactionRefs []string
}

func (s *stateListener) updateIfMatches(ref string, inTransaction bool) {
	if inTransaction {
		s.transactionRefs = append(s.transactionRefs, ref)
		return
	}

	if len(s.filters) == 0 {
		s.callback(context.Background(), ref)
		return
	}

	for _, filter := range s.filters {
		if filter.Match(ref) {
			s.callback(context.Background(), ref)
			return
		}
	}
}

// AddDependency implements RefStore.
func (s *stateListener) AddDependency(ctx context.Context, ref string, dependency string) error {
	return s.store.AddDependency(ctx, ref, dependency)
}

// Close implements RefStore.
func (s *stateListener) Close() error {
	return s.store.Close()
}

// Delete implements RefStore.
func (s *stateListener) Delete(ctx context.Context, ref string) error {
	s.updateIfMatches(ref, s.inTransaction)
	return s.store.Delete(ctx, ref)
}

// Get implements RefStore.
func (s *stateListener) Get(ctx context.Context, ref string, v any) error {
	return s.store.Get(ctx, ref, v)
}

// GetDependants implements RefStore.
func (s *stateListener) GetDependants(ctx context.Context, ref string) ([]string, error) {
	return s.store.GetDependants(ctx, ref)
}

// GetDependencies implements RefStore.
func (s *stateListener) GetDependencies(ctx context.Context, ref string) ([]string, error) {
	return s.store.GetDependencies(ctx, ref)
}

// GetLinks implements RefStore.
func (s *stateListener) GetLinks(ctx context.Context, ref string) ([]string, error) {
	return s.store.GetLinks(ctx, ref)
}

// Link implements RefStore.
func (s *stateListener) Link(ctx context.Context, ref string, target string) error {
	return s.store.Link(ctx, ref, target)
}

// Unlink implements RefStore.
func (s *stateListener) Unlink(ctx context.Context, ref string) error {
	return s.store.Unlink(ctx, ref)
}

// Match implements RefStore.
func (s *stateListener) Match(ctx context.Context, glob ...string) ([]string, error) {
	return s.store.Match(ctx, glob...)
}

// RemoveDependency implements RefStore.
func (s *stateListener) RemoveDependency(ctx context.Context, ref string, dependency string) error {
	return s.store.RemoveDependency(ctx, ref, dependency)
}

// ResolveLink implements RefStore.
func (s *stateListener) ResolveLink(ctx context.Context, ref string) (string, error) {
	return s.store.ResolveLink(ctx, ref)
}

// Set implements RefStore.
func (s *stateListener) Set(ctx context.Context, ref string, v any) error {
	s.updateIfMatches(ref, s.inTransaction)
	return s.store.Set(ctx, ref, v)
}

// StartTransaction implements RefStore.
func (s *stateListener) StartTransaction(ctx context.Context) error {
	s.inTransaction = true
	return s.store.StartTransaction(ctx)
}

// CommitTransaction implements RefStore.
func (s *stateListener) CommitTransaction(ctx context.Context, message string) error {
	for _, ref := range s.transactionRefs {
		s.updateIfMatches(ref, false)
	}
	s.inTransaction = false
	s.transactionRefs = nil
	return s.store.CommitTransaction(ctx, message)
}
