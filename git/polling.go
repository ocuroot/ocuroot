package git

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/log"
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
	type callBackConfig struct {
		endpoint string
		hash     string
	}

	callbacks := make(chan callBackConfig)

	// Each target needs a dedicated ticker so they all respond to every tick
	tickerByTarget := make(map[PollTarget]chan time.Time)

	for _, target := range targets {
		remote, err := NewRemoteGit(target.Endpoint)
		if err != nil {
			return err
		}

		tickerByTarget[target] = make(chan time.Time)

		go PollRemote(ctx, remote, target.Branch, func(hash string) {
			callbacks <- callBackConfig{
				endpoint: target.Endpoint,
				hash:     hash,
			}
		}, tickerByTarget[target])
	}

	// Forward ticker to all targets
	go func() {
		for t := range ticker {
			for _, t2 := range tickerByTarget {
				t2 <- t
			}
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return nil
		case config := <-callbacks:
			callback(config.endpoint, config.hash)
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
	if !strings.HasPrefix(branch, "refs/") {
		branch = fmt.Sprintf("refs/heads/%v", branch)
	}

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
				// TODO: Maybe randomize a retry
				log.Error("Error getting current hash", "branch", branch, "endpoint", remote.Endpoint(), "err", err)
				continue
			}

			if currentHash != lastHash {
				log.Info("Branch hash changed", "endpoint", remote.Endpoint(), "branch", branch, "lastHash", lastHash, "currentHash", currentHash)
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
