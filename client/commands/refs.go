package commands

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/charmbracelet/log"
	"github.com/ocuroot/ocuroot/client"
	"github.com/ocuroot/ocuroot/client/local"
	"github.com/ocuroot/ocuroot/client/release"
	"github.com/ocuroot/ocuroot/refs"
	"github.com/ocuroot/ocuroot/refs/refstore"
	"github.com/ocuroot/ocuroot/sdk"
	"github.com/ocuroot/ocuroot/store/models"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"go.starlark.net/starlark"
)

func GetRef(cmd *cobra.Command, args []string) (refs.Ref, error) {
	if len(args) > 0 {
		return refs.Parse(args[0])
	}

	out := refs.Ref{}
	if cmd != nil {
		out.Filename, _ = cmd.Flags().GetString("package")
		releaseName, _ := cmd.Flags().GetString("release")
		if releaseName != "" {
			out = out.SetRelease(releaseName)
		}
	}

	if out.Filename == "" {
		out.Filename = "."
	}

	return out, nil
}

func AddRefFlags(cmd *cobra.Command, persistent bool) {
	var flags *pflag.FlagSet
	if persistent {
		flags = cmd.PersistentFlags()
	} else {
		flags = cmd.Flags()
	}
	flags.String("package", ".", "Path to the working package in the current repository. Can also be specified via a full ref in the first parameter.")
	flags.String("release", "", "ID or tag of the release. Can also be specified via a full ref in the first parameter.")
}

func storeFromRepoOrStateRoot(ctx context.Context) (store refstore.Store, isRepo bool, err error) {
	wd, err := os.Getwd()
	if err != nil {
		return nil, false, err
	}

	// Get a read only store from the state root
	stateRootPath, err := client.FindStateStoreRoot(wd)
	if err == nil {
		fs, err := refstore.NewFSRefStore(stateRootPath)
		if err != nil {
			return nil, false, fmt.Errorf("failed to create fs ref store: %w", err)
		}
		return fs, false, nil
	}

	log.Info("No state root found, using repo root", "wd", wd)

	// Get a read only store from the repo root if available
	repoRootPath, err := client.FindRepoRoot(wd)
	if err != nil && !errors.Is(err, client.ErrRootNotFound) {
		return nil, false, fmt.Errorf("failed to find repo root: %w", err)
	}
	if errors.Is(err, client.ErrRootNotFound) {
		return nil, false, errors.New("a repo.ocu.star file is required in the root of your project")
	}

	s, _, err := loadStoreFromRepoRoot(ctx, repoRootPath)
	if err != nil {
		return nil, false, fmt.Errorf("failed to load store from repo root: %w", err)
	}
	return s, true, nil
}

// Get a read/write store by loading repo config
func getReadOnlyStore(ctx context.Context) (refstore.Store, error) {
	store, _, err := storeFromRepoOrStateRoot(ctx)
	if err != nil {
		return nil, err
	}
	return refstore.NewReadOnlyStore(store), nil
}

func loadStoreFromRepoRoot(ctx context.Context, repoRootPath string) (refstore.Store, refstore.Store, error) {
	// Create a backend that is just enough for loading repo config
	backend, be := local.BackendForRepo()

	_, err := sdk.LoadRepo(
		ctx,
		sdk.NewFSResolver(os.DirFS(repoRootPath)),
		"repo.ocu.star",
		backend,
		func(thread *starlark.Thread, msg string) {
			log.Info(msg)
		},
	)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to load repo: %w", err)
	}

	var repoURL string = be.RepoAlias
	if repoURL == "" {
		var err error
		repoURL, err = client.GetRepoURL(repoRootPath)
		if err != nil {
			return nil, nil, err
		}
	}

	state, intent, err := release.NewRefStore(
		be.Store,
		repoURL,
		repoRootPath,
	)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create ref store: %w", err)
	}

	return state, intent, nil
}

func getTrackerConfig(ctx context.Context, cmd *cobra.Command, args []string) (release.TrackerConfig, error) {
	wd, err := os.Getwd()
	if err != nil {
		return release.TrackerConfig{}, err
	}

	repoRootPath, err := client.FindRepoRoot(wd)
	if err != nil && !errors.Is(err, client.ErrRootNotFound) {
		return release.TrackerConfig{}, err
	}
	if errors.Is(err, client.ErrRootNotFound) {
		return release.TrackerConfig{}, errors.New("a repo.ocu.star file is required in the root of your project")
	}

	// Create a backend that is just enough for loading repo config
	backend, be := local.BackendForRepo()

	data, err := sdk.LoadRepo(
		ctx,
		sdk.NewFSResolver(os.DirFS(repoRootPath)),
		"repo.ocu.star",
		backend,
		func(thread *starlark.Thread, msg string) {
			log.Info(msg)
		},
	)
	if err != nil {
		return release.TrackerConfig{}, fmt.Errorf("failed to load repo: %w", err)
	}

	ref, err := GetRef(cmd, args)
	if err != nil {
		return release.TrackerConfig{}, err
	}

	if ref.IsRelative() {
		wdRel, err := filepath.Rel(repoRootPath, wd)
		if err != nil {
			return release.TrackerConfig{}, err
		}

		baseRef := refs.Ref{
			Repo:     be.RepoAlias,
			Filename: wdRel,
		}

		if be.RepoAlias == "" {
			repoURL, err := client.GetRepoURL(repoRootPath)
			if err != nil {
				return release.TrackerConfig{}, err
			}
			baseRef.Repo = repoURL
		}
		ref, err = ref.RelativeTo(baseRef)
		if err != nil {
			return release.TrackerConfig{}, err
		}
	}

	state, intent, err := release.NewRefStore(
		be.Store,
		ref.Repo,
		repoRootPath,
	)
	if err != nil {
		return release.TrackerConfig{}, fmt.Errorf("failed to create ref store: %w", err)
	}

	commit, err := client.GetRepoCommit(repoRootPath)
	if err != nil {
		return release.TrackerConfig{}, fmt.Errorf("failed to get repo commit: %w", err)
	}

	tc := release.TrackerConfig{
		Commit:      commit,
		RepoPath:    repoRootPath,
		Ref:         ref,
		State:       state,
		Intent:      intent,
		StoreConfig: be.Store,
	}

	err = saveRepoConfig(ctx, tc, data)
	if err != nil {
		return release.TrackerConfig{}, fmt.Errorf("failed to save repo config: %w", err)
	}

	return tc, nil
}

func saveRepoConfig(ctx context.Context, tc release.TrackerConfig, data []byte) (err error) {
	// Write the repo file to the state stores for later use
	repoRef := tc.Ref.
		SetFilename("repo.ocu.star").
		SetRelease(tc.Commit).
		SetSubPathType(refs.SubPathTypeNone).
		SetSubPath("").
		SetFragment("")

	repoConfig := models.RepoConfig{
		Source: data,
	}

	err = tc.State.StartTransaction(ctx, "Save repo config")
	if err != nil {
		return fmt.Errorf("failed to start transaction: %w", err)
	}
	err = tc.Intent.StartTransaction(ctx, "Save repo config")
	if err != nil {
		return fmt.Errorf("failed to start transaction: %w", err)
	}

	defer func() {
		// TODO: We need a way to revert a transaction
		if err == nil {
			stateErr := tc.State.CommitTransaction(ctx)
			intentErr := tc.Intent.CommitTransaction(ctx)
			if stateErr != nil && intentErr != nil {
				err = fmt.Errorf("failed to commit transaction: %w, %w", stateErr, intentErr)
			} else if stateErr != nil {
				err = fmt.Errorf("failed to commit transaction: %w", stateErr)
			} else if intentErr != nil {
				err = fmt.Errorf("failed to commit transaction: %w", intentErr)
			}
		}
	}()

	if err := tc.State.Get(ctx, repoRef.String(), &models.RepoConfig{}); err == refstore.ErrRefNotFound {
		if err = tc.State.Set(ctx, repoRef.String(), repoConfig); err != nil {
			return fmt.Errorf("failed to set ref store: %w", err)
		}
		if err = tc.State.Link(ctx, repoRef.SetRelease("").String(), repoRef.String()); err != nil {
			return fmt.Errorf("failed to link ref store: %w", err)
		}

		// Add support files if this is the first time we've seen this commit
		if gitSupportFilesBackend, ok := tc.State.(refstore.GitSupportFileWriter); ok && tc.StoreConfig != nil && tc.StoreConfig.State.Git != nil {
			if err := gitSupportFilesBackend.AddSupportFiles(ctx, tc.StoreConfig.State.Git.SupportFiles); err != nil {
				return fmt.Errorf("failed to add support files: %w", err)
			}
		}
	}

	if err := tc.Intent.Get(ctx, repoRef.String(), &models.RepoConfig{}); err == refstore.ErrRefNotFound {
		if err := tc.Intent.Set(ctx, repoRef.String(), repoConfig); err != nil {
			return fmt.Errorf("failed to set ref store: %w", err)
		}
		if err := tc.Intent.Link(ctx, repoRef.SetRelease("").String(), repoRef.String()); err != nil {
			return fmt.Errorf("failed to link ref store: %w", err)
		}

		// Add support files if this is the first time we've seen this commit
		if gitSupportFilesBackend, ok := tc.Intent.(refstore.GitSupportFileWriter); ok &&
			tc.StoreConfig != nil &&
			tc.StoreConfig.Intent != nil &&
			tc.StoreConfig.Intent.Git != nil {
			if err := gitSupportFilesBackend.AddSupportFiles(ctx, tc.StoreConfig.Intent.Git.SupportFiles); err != nil {
				return fmt.Errorf("failed to add support files: %w", err)
			}
		}
	}

	return nil
}
