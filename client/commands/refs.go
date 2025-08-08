package commands

import (
	"context"
	"errors"
	"fmt"
	"os"

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
	out.Filename, _ = cmd.Flags().GetString("package")
	releaseName, _ := cmd.Flags().GetString("release")
	if releaseName != "" {
		out.ReleaseOrIntent = refs.ReleaseOrIntent{
			Type:  refs.Release,
			Value: releaseName,
		}
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

func getTrackerConfigNoRef() (release.TrackerConfig, error) {
	wd, err := os.Getwd()
	if err != nil {
		return release.TrackerConfig{}, err
	}

	repoRootPath, err := client.FindRepoRoot(wd)
	if err != nil {
		return release.TrackerConfig{}, err
	}

	repoURL, commit, err := client.GetRepoInfo(repoRootPath)
	if err != nil {
		return release.TrackerConfig{}, err
	}

	// Create a backend that is just enough for loading repo config
	backend, be := local.BackendForRepo()

	data, err := sdk.LoadRepo(
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

	ref := refs.Ref{
		Repo: repoURL,
	}
	if be.RepoAlias != "" {
		ref.Repo = be.RepoAlias
	}

	s, err := release.NewRefStore(
		be.Store,
		ref.Repo,
		repoRootPath,
	)
	if err != nil {
		return release.TrackerConfig{}, fmt.Errorf("failed to create ref store: %w", err)
	}

	tc := release.TrackerConfig{
		Commit:   commit,
		RepoPath: repoRootPath,
		Ref:      ref,
		Store:    s,
	}

	err = saveRepoConfig(tc, data)
	if err != nil {
		return release.TrackerConfig{}, fmt.Errorf("failed to save repo config: %w", err)
	}

	return tc, nil
}

func execRepoFileFromStore(ctx context.Context, readOnlyStore refstore.Store, repoConfigRef string) (*local.BackendOutputs, error) {
	var repoConfig models.RepoConfig
	if err := readOnlyStore.Get(ctx, repoConfigRef, &repoConfig); err != nil {
		return nil, fmt.Errorf("failed to get repo config: %w", err)
	}

	backend, be := local.BackendForRepo()

	_, err := sdk.LoadRepoFromBytes(
		sdk.NewNullResolver(),
		"repo.ocu.star",
		repoConfig.Source,
		backend,
		func(thread *starlark.Thread, msg string) {},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to load repo: %w", err)
	}
	return be, nil
}

func getReadWriteStore() (refstore.Store, error) {
	rw, isRepo, err := storeFromRepoOrStateRoot()
	if err != nil {
		return nil, err
	}

	if isRepo {
		return rw, nil
	}

	allRepoConfigs, err := rw.Match(context.Background(), "**/repo.ocu.star/{+,@}*")
	if err != nil {
		return nil, err
	}

	for _, repoConfig := range allRepoConfigs {
		fmt.Println(repoConfig)
		be, err := execRepoFileFromStore(context.Background(), rw, repoConfig)
		if err != nil {
			return nil, err
		}
		if be.Store == nil {
			continue
		}
		if be.Store.State.Git == nil {
			continue
		}
		return release.NewRefStore(
			be.Store,
			repoConfig,
			"",
		)
	}

	return nil, fmt.Errorf("no git store found")
}

func storeFromRepoOrStateRoot() (store refstore.Store, isRepo bool, err error) {
	wd, err := os.Getwd()
	if err != nil {
		return nil, false, err
	}

	// Get a read only store from the repo root if available
	repoRootPath, err := client.FindRepoRoot(wd)
	if err != nil && !errors.Is(err, client.ErrRootNotFound) {
		return nil, false, err
	}
	if err == nil {
		s, err := loadStoreFromRepoRoot(repoRootPath)
		if err != nil {
			return nil, false, err
		}
		return s, true, nil
	}

	// Get a read only store from the state root
	stateRootPath, err := client.FindStateStoreRoot(wd)
	if err != nil {
		return nil, false, err
	}

	fs, err := refstore.NewFSRefStore(stateRootPath)
	if err != nil {
		return nil, false, err
	}
	return fs, false, nil
}

// TODO: Get a read/write store by loading repo config
// There should be an alternative function to this one
func getReadOnlyStore() (refstore.Store, error) {
	store, _, err := storeFromRepoOrStateRoot()
	if err != nil {
		return nil, err
	}
	return refstore.NewReadOnlyStore(store), nil
}

func loadStoreFromRepoRoot(repoRootPath string) (refstore.Store, error) {
	// Create a backend that is just enough for loading repo config
	backend, be := local.BackendForRepo()

	_, err := sdk.LoadRepo(
		sdk.NewFSResolver(os.DirFS(repoRootPath)),
		"repo.ocu.star",
		backend,
		func(thread *starlark.Thread, msg string) {
			log.Info(msg)
		},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to load repo: %w", err)
	}

	var repoURL string = be.RepoAlias
	if repoURL == "" {
		var err error
		repoURL, _, err = client.GetRepoInfo(repoRootPath)
		if err != nil {
			return nil, err
		}
	}

	s, err := release.NewRefStore(
		be.Store,
		repoURL,
		repoRootPath,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create ref store: %w", err)
	}

	return s, nil
}

func getTrackerConfig(cmd *cobra.Command, args []string) (release.TrackerConfig, error) {
	wd, err := os.Getwd()
	if err != nil {
		return release.TrackerConfig{}, err
	}

	repoRootPath, err := client.FindRepoRoot(wd)
	if err != nil {
		return release.TrackerConfig{}, err
	}

	repoURL, commit, err := client.GetRepoInfo(repoRootPath)
	if err != nil {
		return release.TrackerConfig{}, err
	}

	ref, err := GetRef(cmd, args)
	if err != nil {
		return release.TrackerConfig{}, err
	}
	if ref.Repo == "" && !ref.Global {
		ref.Repo = repoURL
	}

	// Create a backend that is just enough for loading repo config
	backend, be := local.BackendForRepo()

	data, err := sdk.LoadRepo(
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

	if be.RepoAlias != "" {
		ref.Repo = be.RepoAlias
	}

	s, err := release.NewRefStore(
		be.Store,
		ref.Repo,
		repoRootPath,
	)
	if err != nil {
		return release.TrackerConfig{}, fmt.Errorf("failed to create ref store: %w", err)
	}

	tc := release.TrackerConfig{
		Commit:   commit,
		RepoPath: repoRootPath,
		Ref:      ref,
		Store:    s,
	}

	err = saveRepoConfig(tc, data)
	if err != nil {
		return release.TrackerConfig{}, fmt.Errorf("failed to save repo config: %w", err)
	}

	return tc, nil
}

func saveRepoConfig(tc release.TrackerConfig, data []byte) error {
	// Write the repo file to the state stores for later use
	repoRef := tc.Ref
	repoRef.Filename = "repo.ocu.star"
	repoRef.ReleaseOrIntent.Type = refs.Release
	repoRef.ReleaseOrIntent.Value = tc.Commit
	repoRef.SubPathType = refs.SubPathTypeNone
	repoRef.SubPath = ""
	repoRef.Fragment = ""

	repoConfig := models.RepoConfig{
		Source: data,
	}

	err := tc.Store.StartTransaction(context.Background())
	if err != nil {
		return fmt.Errorf("failed to start transaction: %w", err)
	}
	defer func() {
		tc.Store.CommitTransaction(context.Background(), "Save repo config")
	}()

	if err := tc.Store.Get(context.Background(), repoRef.String(), &models.RepoConfig{}); err == refstore.ErrRefNotFound {
		if err = tc.Store.Set(context.Background(), repoRef.String(), repoConfig); err != nil {
			return fmt.Errorf("failed to set ref store: %w", err)
		}
		if err = tc.Store.Link(context.Background(), repoRef.SetVersion("").String(), repoRef.String()); err != nil {
			return fmt.Errorf("failed to link ref store: %w", err)
		}
	}

	repoRef = repoRef.MakeIntent()
	if err := tc.Store.Get(context.Background(), repoRef.String(), &models.RepoConfig{}); err == refstore.ErrRefNotFound {
		if err := tc.Store.Set(context.Background(), repoRef.String(), repoConfig); err != nil {
			return fmt.Errorf("failed to set ref store: %w", err)
		}
		if err := tc.Store.Link(context.Background(), repoRef.SetVersion("").String(), repoRef.String()); err != nil {
			return fmt.Errorf("failed to link ref store: %w", err)
		}
	}

	return nil
}
