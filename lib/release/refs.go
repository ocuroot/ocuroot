package release

import (
	"context"
	"strings"

	libglob "github.com/gobwas/glob"
	"github.com/ocuroot/ocuroot/refs"
	"github.com/ocuroot/ocuroot/refs/refstore"
	"github.com/ocuroot/ocuroot/sdk"
	"github.com/ocuroot/ocuroot/store/models"
)

var (
	GlobPackage     = libglob.MustCompile(`**/[^@]+/**`, '/')
	GlobRepoConfig  = libglob.MustCompile("**/-/repo.ocu.star/@*", '/')
	GlobRelease     = libglob.MustCompile("**/@*", '/')
	GlobTask        = libglob.MustCompile("**/@*/{task,deploy}/*", '/')
	GlobOp          = libglob.MustCompile("**/@*/op/*", '/')
	GlobDeployment  = libglob.MustCompile("**/@*/deploy/*", '/')
	GlobRun         = libglob.MustCompile("**/@*/{task,deploy}/*/*", '/')
	GlobLog         = libglob.MustCompile("**/@*/{task,deploy}/*/*/logs", '/')
	GlobCustom      = libglob.MustCompile("**/@*/custom/*", '/')
	GlobEnvironment = libglob.MustCompile("@*/environment/*", '/')
)

func ReduceToReleaseConfig(ref string) string {
	return strings.Split(ref, "@")[0]
}

func ReduceToTaskRef(ref refs.Ref) (refs.Ref, error) {
	wr, err := refs.Reduce(ref.String(), GlobTask)
	if err != nil {
		return ref, err
	}
	out, err := refs.Parse(wr)
	if err != nil {
		return ref, err
	}
	return out, nil
}

func ReduceToRunRef(ref refs.Ref) refs.Ref {
	wr, err := refs.Reduce(ref.String(), GlobRun)
	if err != nil {
		return ref
	}
	out, err := refs.Parse(wr)
	if err != nil {
		return ref
	}
	return out
}

type Custom any

// LoadRef loads the document at a reference and
func LoadRef(ctx context.Context, store refstore.Store, ref refs.Ref) (any, error) {
	switch {
	case GlobRepoConfig.Match(ref.String()):
		return LoadRefOfType[models.RepoConfig](ctx, store, ref)
	case GlobRelease.Match(ref.String()):
		return LoadRefOfType[ReleaseInfo](ctx, store, ref)
	case GlobTask.Match(ref.String()):
		return LoadRefOfType[models.Run](ctx, store, ref)
	case GlobRun.Match(ref.String()):
		return LoadRefOfType[models.Run](ctx, store, ref)
	case GlobLog.Match(ref.String()):
		return LoadRefOfType[[]sdk.Log](ctx, store, ref)
	case GlobEnvironment.Match(ref.String()):
		return LoadRefOfType[models.Environment](ctx, store, ref)
	default:
		return LoadRefOfType[any](ctx, store, ref)
	}
}

func LoadRefOfType[T any](ctx context.Context, store refstore.Store, ref refs.Ref) (T, error) {
	var out T
	if err := store.Get(ctx, ref.String(), &out); err != nil {
		return out, err
	}
	return out, nil
}
