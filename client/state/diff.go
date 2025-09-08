package state

import (
	"context"
	"fmt"
	"reflect"

	"github.com/charmbracelet/log"
	"github.com/ocuroot/ocuroot/refs"
	"github.com/ocuroot/ocuroot/refs/refstore"
	"github.com/ocuroot/ocuroot/store/models"
)

func Diff(ctx context.Context, state, intent refstore.Store) ([]string, error) {
	stateRefs, err := state.Match(ctx, "**/@/{deploy}/*", "**/@*/custom/*", "@/{custom,environment}/*")
	if err != nil {
		return nil, fmt.Errorf("failed to match state refs: %w", err)
	}

	intentRefs, err := intent.Match(ctx, "**/@/{deploy}/*", "**/@*/custom/*", "@/{custom,environment}/*")
	if err != nil {
		return nil, fmt.Errorf("failed to match intent refs: %w", err)
	}

	log.Debug("Diffing", "stateRefs", stateRefs, "intentRefs", intentRefs)

	var (
		stateToIntentRefSet = make(map[string]string)
	)

	var diffs []string

	for _, ref := range stateRefs {
		pr, err := refs.Parse(ref)
		if err != nil {
			return nil, fmt.Errorf("failed to parse state ref: %w", err)
		}
		stateToIntentRefSet[ref] = pr.String()
	}

	for _, ref := range intentRefs {
		ir, err := refs.Parse(ref)
		if err != nil {
			return nil, fmt.Errorf("failed to parse intent ref: %w", err)
		}
		resolvedRef, err := state.ResolveLink(ctx, ir.String())
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

	log.Debug("After matching", "stateToIntentRefSet", stateToIntentRefSet, "diffs", diffs)

	for stateRef, intentRef := range stateToIntentRefSet {
		ir, err := refs.Parse(intentRef)
		if err != nil {
			return nil, fmt.Errorf("failed to parse intent ref: %w", err)
		}
		sr, err := refs.Parse(stateRef)
		if err != nil {
			return nil, fmt.Errorf("failed to parse state ref: %w", err)
		}
		match, err := compareIntent(ctx, state, intent, ir, sr)
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
	state, intent refstore.Store,
	intentRef refs.Ref,
	stateRef refs.Ref,
) (bool, error) {
	switch intentRef.SubPathType {
	case refs.SubPathTypeCustom:
		return compareExplicitIntent(ctx, state, intent, intentRef, stateRef)
	case refs.SubPathTypeDeploy:
		return compareDeployIntent(ctx, state, intent, intentRef, stateRef)
	case refs.SubPathTypeEnvironment:
		return compareExplicitIntent(ctx, state, intent, intentRef, stateRef)
	}
	return false, fmt.Errorf("unsupported subpath type: %s", intentRef.SubPathType)
}

// compareExplicitIntent compares the contents of the refs directly.
// This is used for state that should match exactly between intent and state.
func compareExplicitIntent(
	ctx context.Context,
	state, intent refstore.Store,
	intentRef refs.Ref,
	stateRef refs.Ref,
) (bool, error) {
	var intentContent, stateContent any
	if err := intent.Get(ctx, intentRef.String(), &intentContent); err != nil {
		if err == refstore.ErrRefNotFound {
			// It's ok if the state exists but the intent doesn't
			return false, nil
		}
		return false, fmt.Errorf("failed to get intent content: %w", err)
	}
	if err := state.Get(ctx, stateRef.String(), &stateContent); err != nil {
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
	state, intent refstore.Store,
	intentRef refs.Ref,
	stateRef refs.Ref,
) (bool, error) {
	var (
		intentContent models.Intent
		stateContent  models.Intent
	)
	if err := intent.Get(ctx, intentRef.String(), &intentContent); err != nil {
		if err == refstore.ErrRefNotFound {
			return false, nil
		}
		return false, fmt.Errorf("failed to get intent content: %w", err)
	}
	if err := state.Get(ctx, stateRef.String(), &stateContent); err != nil {
		if err == refstore.ErrRefNotFound {
			return false, nil
		}
		return false, fmt.Errorf("failed to get state content: %w", err)
	}

	log.Debug("Comparing deploy", "intentRef", intentRef.String(), "stateRef", stateRef.String(), "intentContent", intentContent, "stateContent", stateContent)

	if !reflect.DeepEqual(intentContent, stateContent) {
		log.Debug("Not equal")
		return false, nil
	}

	log.Debug("Equal")
	return true, nil
}
