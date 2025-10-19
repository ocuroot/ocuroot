package git

import (
	"context"
	"fmt"
	"time"
)

func PollRemote(
	ctx context.Context,
	remote RemoteGit,
	branch string,
	callback func(hash string),
	ticker <-chan time.Time,
) error {
	lastHash, err := currentHash(ctx, remote, branch)
	if err != nil {
		return err
	}
	callback(lastHash)

	// Invalidate after initial callback
	remote.InvalidateConnection()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case _, ok := <-ticker:
			if !ok {
				// Ticker channel closed, exit cleanly
				return nil
			}

			// Invalidate before polling
			remote.InvalidateConnection()

			currentHash, err := currentHash(ctx, remote, branch)
			if err != nil {
				fmt.Println(time.Now(), "Error getting current hash: ", err)
				return err
			}

			if currentHash != lastHash {
				lastHash = currentHash
				callback(currentHash)
			}
		}
	}
}

func currentHash(ctx context.Context, remote RemoteGit, branch string) (string, error) {
	refs, err := remote.BranchRefs(ctx)
	if err != nil {
		return "", err
	}

	for _, ref := range refs {
		if ref.Name == branch {
			return ref.Hash, nil
		}
	}

	return "", nil
}
