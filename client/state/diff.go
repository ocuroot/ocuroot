package state

import (
	"context"
	"fmt"
	"reflect"

	"github.com/charmbracelet/log"
	librelease "github.com/ocuroot/ocuroot/lib/release"
	"github.com/ocuroot/ocuroot/refs"
	"github.com/ocuroot/ocuroot/refs/refstore"
	"github.com/ocuroot/ocuroot/store/models"
)

func Diff(ctx context.Context, store refstore.Store) ([]string, error) {
	stateRefs, err := store.Match(ctx, "**/@/{deploy}/*", "**/@*/custom/*", "@/{custom,environment}/*")
	if err != nil {
		return nil, fmt.Errorf("failed to match state refs: %w", err)
	}

	intentRefs, err := store.Match(ctx, "**/+/{deploy}/*", "**/+*/custom/*", "+/{custom,environment}/*")
	if err != nil {
		return nil, fmt.Errorf("failed to match intent refs: %w", err)
	}

	var (
		stateToIntentRefSet = make(map[string]string)
	)

	var diffs []string

	for _, ref := range stateRefs {
		log.Info("State ref", "ref", ref)
		pr, err := refs.Parse(ref)
		if err != nil {
			return nil, fmt.Errorf("failed to parse state ref: %w", err)
		}
		pr = pr.MakeIntent()
		stateToIntentRefSet[ref] = pr.String()
	}

	for _, ref := range intentRefs {
		log.Info("Intent ref", "ref", ref)
		ir, err := refs.Parse(ref)
		if err != nil {
			return nil, fmt.Errorf("failed to parse intent ref: %w", err)
		}
		ir = ir.MakeRelease()
		resolvedRef, err := store.ResolveLink(ctx, ir.String())
		if err != nil {
			return nil, fmt.Errorf("failed to resolve intent ref: %w", err)
		}

		if _, exists := stateToIntentRefSet[resolvedRef]; exists {
			// Update ref to make sure we capture links
			stateToIntentRefSet[resolvedRef] = ref
			continue
		}

		// Intent ref doesn't exist, may need to remove
		diffs = append(diffs, ref)
	}

	for stateRef, intentRef := range stateToIntentRefSet {
		ir, err := refs.Parse(intentRef)
		if err != nil {
			return nil, fmt.Errorf("failed to parse intent ref: %w", err)
		}
		sr, err := refs.Parse(stateRef)
		if err != nil {
			return nil, fmt.Errorf("failed to parse state ref: %w", err)
		}
		match, err := compareIntent(ctx, store, ir, sr)
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
	stateRef refs.Ref,
) (bool, error) {
	switch intentRef.SubPathType {
	case refs.SubPathTypeCustom:
		return compareExplicitIntent(ctx, store, intentRef, stateRef)
	case refs.SubPathTypeDeploy:
		return compareDeployIntent(ctx, store, intentRef, stateRef)
	case refs.SubPathTypeEnvironment:
		return compareExplicitIntent(ctx, store, intentRef, stateRef)
	}
	return false, fmt.Errorf("unsupported subpath type: %s", intentRef.SubPathType)
}

// compareExplicitIntent compares the contents of the refs directly.
// This is used for state that should match exactly between intent and state.
func compareExplicitIntent(
	ctx context.Context,
	store refstore.Store,
	intentRef refs.Ref,
	stateRef refs.Ref,
) (bool, error) {
	var intentContent, stateContent any
	if err := store.Get(ctx, intentRef.String(), &intentContent); err != nil {
		if err == refstore.ErrRefNotFound {
			// It's ok if the state exists but the intent doesn't
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

	if !reflect.DeepEqual(intentContent, stateContent) {
		return false, nil
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
