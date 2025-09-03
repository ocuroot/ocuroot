package release

import (
	"context"

	libglob "github.com/gobwas/glob"
	"github.com/ocuroot/ocuroot/refs"
	"github.com/ocuroot/ocuroot/refs/refstore"
	"github.com/ocuroot/ocuroot/sdk"
	"github.com/ocuroot/ocuroot/store/models"
)

var (
	GlobPackage                 = libglob.MustCompile(`**/[^@+]+/**`, '/')
	GlobRepoConfig              = libglob.MustCompile("**/-/repo.ocu.star/@*", '/')
	GlobRelease                 = libglob.MustCompile("**/{@,+}*", '/')
	GlobWork                    = libglob.MustCompile("**/@*/{call,deploy}/*", '/')
	GlobTask                    = libglob.MustCompile("**/@*/task/*", '/')
	GlobDeploymentState         = libglob.MustCompile("**/@*/deploy/*", '/')
	GlobDeploymentIntent        = libglob.MustCompile("**/+*/deploy/*", '/')
	GlobDeploymentStateOrIntent = libglob.MustCompile("**/{@,+}*/deploy/*", '/')
	GlobCall                    = libglob.MustCompile("**/{@,+}*/call/*", '/')
	GlobJob                     = libglob.MustCompile("**/{@,+}*/{call,deploy}/*/*", '/')
	GlobLog                     = libglob.MustCompile("**/{@,+}*/{call,deploy}/*/*/logs", '/')
	GlobCustomState             = libglob.MustCompile("**/@*/custom/*", '/')
	GlobCustomIntent            = libglob.MustCompile("**/+*/custom/*", '/')
	GlobCustomStateOrIntent     = libglob.MustCompile("**/{@,+}*/custom/*", '/')
	GlobEnvironment             = libglob.MustCompile("{@,+}*/environment/*", '/')
)

func WorkRefFromChainRef(ref refs.Ref) (refs.Ref, error) {
	wr, err := refs.Reduce(ref.String(), GlobWork)
	if err != nil {
		return ref, err
	}
	out, err := refs.Parse(wr)
	if err != nil {
		return ref, err
	}
	return out, nil
}

func ReduceToJobRef(ref refs.Ref) refs.Ref {
	wr, err := refs.Reduce(ref.String(), GlobJob)
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
	case GlobWork.Match(ref.String()):
		return LoadRefOfType[models.Work](ctx, store, ref)
	case GlobJob.Match(ref.String()):
		return LoadRefOfType[models.Work](ctx, store, ref)
	case GlobDeploymentIntent.Match(ref.String()):
		return LoadRefOfType[models.Intent](ctx, store, ref)
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
