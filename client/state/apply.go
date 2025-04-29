package state

import (
	"context"
	"fmt"
	"path"
	"strings"

	"github.com/charmbracelet/log"
	"github.com/ocuroot/ocuroot/client/release"
	librelease "github.com/ocuroot/ocuroot/lib/release"
	"github.com/ocuroot/ocuroot/refs"
	"github.com/ocuroot/ocuroot/refs/refstore"
	"github.com/ocuroot/ocuroot/sdk"
	"github.com/ocuroot/ocuroot/store/models"
)

func ApplyIntent(ctx context.Context, tc release.TrackerConfig) error {
	log.Info("Applying intent", "ref", tc.Ref.String())

	ref := tc.Ref

	if ref.ReleaseOrIntent.Type == refs.Release {
		return fmt.Errorf("ref is not an intent")
	}

	if ref.SubPathType == refs.SubPathTypeCustom || ref.SubPathType == refs.SubPathTypeEnvironment {
		err := applyStaticIntent(ctx, tc)
		if err != nil {
			return fmt.Errorf("failed to apply static intent: %w", err)
		}
	} else {
		err := applyDeployIntent(ctx, tc)
		if err != nil {
			return fmt.Errorf("failed to apply deploy intent: %w", err)
		}
	}
	return nil
}

func applyStaticIntent(ctx context.Context, tc release.TrackerConfig) error {
	ref := tc.Ref
	store := tc.Store

	var content any
	if err := store.Get(ctx, ref.String(), &content); err != nil {
		return fmt.Errorf("failed to get intent: %w", err)
	}

	stateRef := ref.MakeRelease().SetVersion(ref.ReleaseOrIntent.Value)
	if err := store.Set(ctx, stateRef.String(), content); err != nil {
		return fmt.Errorf("failed to set state: %w", err)
	}

	return nil
}

func applyDeletedDeployIntent(ctx context.Context, tc release.TrackerConfig) error {
	tc.Ref = tc.Ref.MakeRelease()
	if tc.Ref.SubPathType != refs.SubPathTypeDeploy {
		return fmt.Errorf("deployment ID must be a deployment ref")
	}
	if strings.Contains(tc.Ref.SubPath, "/") {
		return fmt.Errorf("deployment ID must not contain a slash")
	}

	envName := tc.Ref.SubPath

	releaseStore, err := librelease.ReleaseStore(ctx, tc.Ref.String(), tc.Store)
	if err != nil {
		return fmt.Errorf("failed to get release store: %w", err)
	}

	err = releaseStore.InitDeployment(ctx, envName, true)
	if err != nil {
		return fmt.Errorf("failed to init deployment: %w", err)
	}

	return nil
}

func applyDeployIntent(ctx context.Context, tc release.TrackerConfig) error {
	ref := tc.Ref
	store := tc.Store

	var intentContent models.Intent
	if err := store.Get(ctx, ref.String(), &intentContent); err != nil {
		if err == refstore.ErrRefNotFound {
			return applyDeletedDeployIntent(ctx, tc)
		}
		return fmt.Errorf("failed to get intent: %w", err)
	}

	var releaseSummary models.ReleaseSummary
	if err := store.Get(ctx, intentContent.Release.String(), &releaseSummary); err != nil {
		return fmt.Errorf("failed to get release summary: %w", err)
	}

	var releaseInfo librelease.ReleaseInfo
	if err := store.Get(ctx, intentContent.Release.String(), &releaseInfo); err != nil {
		return fmt.Errorf("failed to get release info: %w", err)
	}

	var deployment *sdk.Deployment
	expectedEnvironment := sdk.EnvironmentName(strings.SplitAfter(ref.SubPath, "/")[0])
	for _, p := range releaseInfo.Package.Phases {
		for _, w := range p.Work {
			if w.Deployment == nil || w.Deployment.Environment != expectedEnvironment {
				continue
			}
			deployment = w.Deployment
		}
	}

	if deployment == nil {
		return fmt.Errorf("deployment config not found for environment %s", expectedEnvironment)
	}

	deployRef := intentContent.Release.
		SetSubPathType(refs.SubPathTypeDeploy).
		SetSubPath(
			path.Join(
				string(expectedEnvironment),
			),
		)

	// Check that there is a change to apply
	match, err := compareDeployIntent(ctx, store, ref, deployRef)
	if err != nil {
		return fmt.Errorf("failed to compare deploy intent: %w", err)
	}
	if match {
		fmt.Println("Intent already applied")
		return nil
	}

	// Check that there isn't already a deployment pending or in progress to apply this config
	matchStr := deployRef.String() + "/*/status/{pending,running}"
	existingDeployments, err := store.Match(ctx, matchStr)
	if err != nil {
		return fmt.Errorf("failed to match pending deployments: %w", err)
	}

	if len(existingDeployments) > 0 {
		for _, existingDeployment := range existingDeployments {
			existingDeployment = strings.TrimSuffix(existingDeployment, "/status/pending")
			existingDeployment = strings.TrimSuffix(existingDeployment, "/status/running")

			existingDeploymentRef, err := refs.Parse(existingDeployment)
			if err != nil {
				return fmt.Errorf("failed to parse existing deployment: %w", err)
			}

			match, err := compareDeployIntent(ctx, store, ref, existingDeploymentRef)
			if err != nil {
				return fmt.Errorf("failed to compare deploy intent: %w", err)
			}
			if match {
				fmt.Println("Deploy intent already pending or in progress")
				return nil
			}
		}
	}

	rs, err := librelease.ReleaseStore(ctx, intentContent.Release.String(), store)
	if err != nil {
		return fmt.Errorf("failed to initialize release store: %w", err)
	}

	chainRefString, err := refstore.IncrementPath(ctx, store, fmt.Sprintf("%s/", deployRef.String()))
	if err != nil {
		return fmt.Errorf("failed to increment path: %w", err)
	}
	chainRef, err := refs.Parse(chainRefString)
	if err != nil {
		return fmt.Errorf("failed to parse chain ref: %w", err)
	}
	err = rs.InitializeFunction(ctx, models.Work{
		Release:    intentContent.Release,
		Entrypoint: chainRef,
	}, chainRef, &models.FunctionSummary{
		ID:     "1",
		Fn:     deployment.Up,
		Inputs: intentContent.Inputs,
		Status: models.SummarizedStatusPending,
	})
	if err != nil {
		return fmt.Errorf("failed to initialize function: %w", err)
	}

	fmt.Println("Created work:", chainRef.String())

	return nil
}
