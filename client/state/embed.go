package state

import (
	_ "embed"

	"github.com/ocuroot/ui/css"
)

//go:embed style.css
var CSS []byte

func init() {
	css.Default().Add(CSS)
}
