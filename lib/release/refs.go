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
	GlobRelease                 = libglob.MustCompile("**/{@,+}*", '/')
	GlobWork                    = libglob.MustCompile("**/@*/{call,deploy}/*", '/')
	GlobTask                    = libglob.MustCompile("**/@*/task/*", '/')
	GlobDeploymentState         = libglob.MustCompile("**/@*/deploy/*", '/')
	GlobDeploymentIntent        = libglob.MustCompile("**/+*/deploy/*", '/')
	GlobDeploymentStateOrIntent = libglob.MustCompile("**/{@,+}*/deploy/*", '/')
	GlobCall                    = libglob.MustCompile("**/{@,+}*/call/*", '/')
	GlobChain                   = libglob.MustCompile("**/{@,+}*/{call,deploy}/*/*", '/')
	GlobFunction                = libglob.MustCompile("**/{@,+}*/{call,deploy}/*/*/functions/*", '/')
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

func ChainRefFromFunctionRef(ref refs.Ref) refs.Ref {
	wr, err := refs.Reduce(ref.String(), GlobChain)
	if err != nil {
		return ref
	}
	out, err := refs.Parse(wr)
	if err != nil {
		return ref
	}
	return out
}

func FunctionRefFromChainRef(ref refs.Ref, fn *models.Function) refs.Ref {
	return ref.JoinSubPath("functions", string(fn.ID))
}

type Custom any

// LoadRef loads the document at a reference and
func LoadRef(ctx context.Context, store refstore.Store, ref refs.Ref) (any, error) {
	switch {
	case GlobRelease.Match(ref.String()):
		return LoadRefOfType[ReleaseInfo](ctx, store, ref)
	case GlobWork.Match(ref.String()):
		return LoadRefOfType[models.Work](ctx, store, ref)
	case GlobChain.Match(ref.String()):
		return LoadRefOfType[models.Work](ctx, store, ref)
	case GlobDeploymentIntent.Match(ref.String()):
		return LoadRefOfType[models.Intent](ctx, store, ref)
	case GlobLog.Match(ref.String()):
		return LoadRefOfType[[]sdk.Log](ctx, store, ref)
	case GlobFunction.Match(ref.String()):
		return LoadRefOfType[FunctionState](ctx, store, ref)
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
