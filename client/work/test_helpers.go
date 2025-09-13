package work

import "github.com/ocuroot/ocuroot/refs"

func mustParseRef(refStr string) refs.Ref {
	ref, err := refs.Parse(refStr)
	if err != nil {
		panic(err)
	}
	return ref
}
