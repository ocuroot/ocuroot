package refstore

import (
	"context"
	"fmt"
	"strconv"
	"strings"
)

func IncrementPath(ctx context.Context, store Store, pathPrefix string) (string, error) {
	matches, err := store.Match(ctx, fmt.Sprintf("%s*", pathPrefix))
	if err != nil {
		return "", fmt.Errorf("failed to match prefix: %w", err)
	}

	if len(matches) == 0 {
		return fmt.Sprintf("%s1", pathPrefix), nil
	}

	var maxVersion int
	for _, match := range matches {
		version := strings.Replace(match, pathPrefix, "", 1)
		versionInt, err := strconv.Atoi(version)
		if err != nil {
			// Assume this is a non-numeric value
			continue
		}
		if versionInt > maxVersion {
			maxVersion = versionInt
		}
	}

	return fmt.Sprintf("%s%d", pathPrefix, maxVersion+1), nil
}
