package state

import (
	"context"
	"fmt"
	"path"
	"strings"

	"github.com/charmbracelet/log"
	librelease "github.com/ocuroot/ocuroot/lib/release"
	"github.com/ocuroot/ocuroot/refs"
	"github.com/ocuroot/ocuroot/refs/refstore"
	"github.com/ocuroot/ocuroot/sdk"
	"github.com/ocuroot/ocuroot/store/models"
	"github.com/ocuroot/ocuroot/ui/components/pipeline"
)

func ApplyIntent(ctx context.Context, ref refs.Ref, store refstore.Store) error {
	log.Info("Applying intent", "ref", ref.String())

	if ref.ReleaseOrIntent.Type == refs.Release {
		return fmt.Errorf("ref is not an intent")
	}

	switch ref.SubPathType {
	case refs.SubPathTypeCustom:
		err := applyCustomIntent(ctx, ref, store)
		if err != nil {
			return fmt.Errorf("failed to apply custom intent: %w", err)
		}
	case refs.SubPathTypeEnvironment:
		err := applyEnvironmentIntent(ctx, ref, store)
		if err != nil {
			return fmt.Errorf("failed to apply environment intent: %w", err)
		}
	case refs.SubPathTypeDeploy:
		err := applyDeployIntent(ctx, ref, store)
		if err != nil {
			return fmt.Errorf("failed to apply deploy intent: %w", err)
		}
	default:
		return fmt.Errorf("unknown subpath type: %s", ref.SubPathType)
	}

	return nil
}

func applyEnvironmentIntent(ctx context.Context, ref refs.Ref, store refstore.Store) error {
	log.Info("Applying environment intent", "ref", ref.String())

	var content any
	if err := store.Get(ctx, ref.String(), &content); err != nil {
		if err == refstore.ErrRefNotFound {
			return applyDeletedEnvironmentIntent(ctx, ref, store)
		}
		return fmt.Errorf("failed to get environment intent: %w", err)
	}

	stateRef := ref.MakeRelease()
	if err := store.Set(ctx, stateRef.String(), content); err != nil {
		return fmt.Errorf("failed to set state: %w", err)
	}

	err := store.StartTransaction(ctx)
	if err != nil {
		return fmt.Errorf("failed to start transaction: %w", err)
	}

	defer func() {
		commitErr := store.CommitTransaction(ctx, "scheduling recalculation of deployments")
		if commitErr != nil {
			log.Error("failed to commit transaction", "error", commitErr)
		}
	}()

	// Add a task to all deployed releases
	deployments, err := store.Match(ctx, "**/@/deploy/*")
	if err != nil {
		return fmt.Errorf("failed to match deployments: %w", err)
	}
	for _, deployment := range deployments {
		resolvedDeployment, err := store.ResolveLink(ctx, deployment)
		if err != nil {
			return fmt.Errorf("failed to resolve deployment: %w", err)
		}
		parsedDeployment, err := refs.Parse(resolvedDeployment)
		if err != nil {
			return fmt.Errorf("failed to parse deployment: %w", err)
		}
		parsedDeployment = parsedDeployment.SetSubPathType(refs.SubPathTypeTask).SetSubPath("check_envs")

		if err := store.Set(ctx, parsedDeployment.String(), models.NewMarker()); err != nil {
			return fmt.Errorf("failed to set task: %w", err)
		}
	}

	return nil
}

func applyDeletedEnvironmentIntent(ctx context.Context, ref refs.Ref, store refstore.Store) error {
	stateRef := ref.MakeRelease()

	// Undeploy everything in this environment
	deployments, err := store.Match(ctx, fmt.Sprintf("**/@/deploy/%v", ref.SubPath))
	if err != nil {
		return fmt.Errorf("failed to match deployments: %w", err)
	}
	for _, deployment := range deployments {
		dp, err := refs.Parse(deployment)
		if err != nil {
			return fmt.Errorf("failed to parse deployment: %w", err)
		}
		dp = dp.MakeIntent()
		if err := store.Delete(ctx, dp.String()); err != nil {
			return fmt.Errorf("failed to delete deployment: %w", err)
		}

		err = applyDeletedDeployIntent(ctx, dp, store)
		if err != nil {
			return fmt.Errorf("failed to apply deleted deploy intent: %w", err)
		}
	}

	// Finally, delete the actual environment
	if err := store.Delete(ctx, stateRef.String()); err != nil {
		return fmt.Errorf("failed to delete state: %w", err)
	}

	return nil
}

func applyCustomIntent(ctx context.Context, ref refs.Ref, store refstore.Store) error {
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

func applyDeployIntent(ctx context.Context, ref refs.Ref, store refstore.Store) error {
	var intentContent models.Intent
	if err := store.Get(ctx, ref.String(), &intentContent); err != nil {
		if err == refstore.ErrRefNotFound {
			return applyDeletedDeployIntent(ctx, ref, store)
		}
		return fmt.Errorf("failed to get intent: %w", err)
	}

	var releaseSummary pipeline.ReleaseSummary
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
	}, chainRef, &models.Function{
		ID:     "1",
		Fn:     deployment.Up,
		Inputs: intentContent.Inputs,
		Status: models.StatusPending,
	})
	if err != nil {
		return fmt.Errorf("failed to initialize function: %w", err)
	}

	fmt.Println("Created work:", chainRef.String())

	return nil
}

func applyDeletedDeployIntent(ctx context.Context, ref refs.Ref, store refstore.Store) error {
	ref = ref.MakeRelease()
	if ref.SubPathType != refs.SubPathTypeDeploy {
		return fmt.Errorf("deployment ID must be a deployment ref")
	}
	if strings.Contains(ref.SubPath, "/") {
		return fmt.Errorf("deployment ID must not contain a slash")
	}

	envName := ref.SubPath

	releaseStore, err := librelease.ReleaseStore(ctx, ref.String(), store)
	if err != nil {
		return fmt.Errorf("failed to get release store: %w", err)
	}

	log.Info("Initing deployment down", "env", envName)
	err = releaseStore.InitDeploymentDown(ctx, envName)
	if err != nil {
		return fmt.Errorf("failed to init deployment: %w", err)
	}

	return nil
}
