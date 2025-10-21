package git

import (
	"context"
	"fmt"
	"time"
)

type PollTarget struct {
	Endpoint string
	Branch   string
}

func PollMultiple(
	ctx context.Context,
	targets []PollTarget,
	callback func(endpoint, hash string),
	ticker <-chan time.Time,
) error {
	for _, target := range targets {
		remote, err := NewRemoteGit(target.Endpoint)
		if err != nil {
			return err
		}

		type callBackConfig struct {
			endpoint string
			hash     string
		}

		callbacks := make(chan callBackConfig)

		go PollRemote(ctx, remote, target.Branch, func(hash string) {
			callbacks <- callBackConfig{
				endpoint: target.Endpoint,
				hash:     hash,
			}
		}, ticker)

		for {
			select {
			case <-ctx.Done():
				return nil
			case config := <-callbacks:
				callback(config.endpoint, config.hash)
			}
		}
	}
	return nil
}

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
