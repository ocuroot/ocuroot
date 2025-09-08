package refs

import (
	"errors"
	"path"
	"strings"

	libglob "github.com/gobwas/glob"
)

func Reduce(ref string, glob libglob.Glob) (string, error) {
	for !glob.Match(ref) {
		if ref == "." {
			return "", errors.New("no match")
		}

		if strings.Contains(ref, "@") {
			pr, err := Parse(ref)
			if err != nil {
				return "", err
			}
			if glob.Match(pr.String()) {
				return pr.String(), nil
			}

			noVersion := pr.SetRelease("")
			if glob.Match(noVersion.String()) {
				return noVersion.String(), nil
			}
		}

		ref = path.Dir(ref)
	}
	return ref, nil
}
