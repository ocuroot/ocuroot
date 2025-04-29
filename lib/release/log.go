package release

import (
	"github.com/ocuroot/ocuroot/refs"
	"github.com/ocuroot/ocuroot/sdk"
)

type Logger func(fnRef refs.Ref, log sdk.Log)
