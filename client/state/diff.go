package state

import (
	"context"
	"fmt"
	"reflect"

	librelease "github.com/ocuroot/ocuroot/lib/release"
	"github.com/ocuroot/ocuroot/refs"
	"github.com/ocuroot/ocuroot/refs/refstore"
	"github.com/ocuroot/ocuroot/store/models"
)

func Diff(ctx context.Context, store refstore.Store) ([]string, error) {
	stateRefs, err := store.Match(ctx, "**/@/{deploy,custom,environment}/*")
	if err != nil {
		return nil, fmt.Errorf("failed to match state refs: %w", err)
	}

	intentRefs, err := store.Match(ctx, "**/+/{deploy,custom,environment}/*")
	if err != nil {
		return nil, fmt.Errorf("failed to match intent refs: %w", err)
	}

	var (
		intentToStateRefSet = make(map[string]string)
	)

	var diffs []string

	for _, ref := range stateRefs {
		// Convert ref to intent
		pr, err := refs.Parse(ref)
		if err != nil {
			return nil, fmt.Errorf("failed to parse state ref: %w", err)
		}
		pr = pr.MakeIntent()

		intentToStateRefSet[pr.String()] = ref
	}

	for _, ref := range intentRefs {
		if _, exists := intentToStateRefSet[ref]; exists {
			continue
		}

		// Intent ref doesn't exist, may need to remove
		diffs = append(diffs, ref)
	}

	for intentRef := range intentToStateRefSet {
		ir, err := refs.Parse(intentRef)
		if err != nil {
			return nil, fmt.Errorf("failed to parse intent ref: %w", err)
		}
		match, err := compareIntent(ctx, store, ir)
		if err != nil {
			return nil, fmt.Errorf("failed to compare intent: %w", err)
		}
		if !match {
			diffs = append(diffs, intentRef)
		}
	}

	return diffs, nil
}

func compareIntent(
	ctx context.Context,
	store refstore.Store,
	intentRef refs.Ref,
) (bool, error) {
	stateRef := intentRef
	stateRef.ReleaseOrIntent = refs.ReleaseOrIntent{
		Type: refs.Release,
	}
	if intentRef.SubPathType == refs.SubPathTypeCustom {
		var intentContent, stateContent any
		if err := store.Get(ctx, intentRef.String(), &intentContent); err != nil {
			if err == refstore.ErrRefNotFound {
				return false, nil
			}
			return false, fmt.Errorf("failed to get intent content: %w", err)
		}
		if err := store.Get(ctx, stateRef.String(), &stateContent); err != nil {
			if err == refstore.ErrRefNotFound {
				return false, nil
			}
			return false, nil
		}

		if !reflect.DeepEqual(intentContent, stateContent) {
			return false, nil
		}
	} else if intentRef.SubPathType == refs.SubPathTypeDeploy {
		return compareDeployIntent(ctx, store, intentRef, stateRef)
	} else {
		return false, fmt.Errorf("unsupported subpath type: %s", intentRef.SubPathType)
	}

	return true, nil
}

func compareDeployIntent(
	ctx context.Context,
	store refstore.Store,
	intentRef refs.Ref,
	stateRef refs.Ref,
) (bool, error) {
	var (
		intentContent models.Intent
		stateContent  models.Work
	)
	if err := store.Get(ctx, intentRef.String(), &intentContent); err != nil {
		if err == refstore.ErrRefNotFound {
			return false, nil
		}
		return false, fmt.Errorf("failed to get intent content: %w", err)
	}
	if err := store.Get(ctx, stateRef.String(), &stateContent); err != nil {
		if err == refstore.ErrRefNotFound {
			return false, nil
		}
		return false, fmt.Errorf("failed to get state content: %w", err)
	}

	if intentContent.Release.String() != stateContent.Release.String() {
		return false, nil
	}

	var fn librelease.FunctionState
	if err := store.Get(ctx, stateContent.Entrypoint.String(), &fn); err != nil {
		if err == refstore.ErrRefNotFound {
			return false, nil
		}
		return false, fmt.Errorf("failed to get function summary: %w", err)
	}

	if !reflect.DeepEqual(intentContent.Inputs, fn.Current.Inputs) {
		return false, nil
	}

	return true, nil
}
