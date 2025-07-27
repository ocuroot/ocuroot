package release

import (
	libglob "github.com/gobwas/glob"
	"github.com/ocuroot/ocuroot/refs"
	"github.com/ocuroot/ocuroot/store/models"
)

var (
	GlobRelease  = libglob.MustCompile("**/{@,+}*", '/')
	GlobWork     = libglob.MustCompile("**/{@,+}*/{call,deploy}/*", '/')
	GlobChain    = libglob.MustCompile("**/{@,+}*/{call,deploy}/*/*", '/')
	GlobFunction = libglob.MustCompile("**/{@,+}*/{call,deploy}/*/*/functions/*", '/')
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

func FunctionRefFromChainRef(ref refs.Ref, fn *models.FunctionSummary) refs.Ref {
	return ref.JoinSubPath("functions", string(fn.ID))
}
