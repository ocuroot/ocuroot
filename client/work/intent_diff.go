package work

import (
	"context"
	"fmt"
	"reflect"

	"github.com/charmbracelet/log"
	"github.com/ocuroot/ocuroot/refs"
	"github.com/ocuroot/ocuroot/refs/refstore"
	"github.com/ocuroot/ocuroot/store/models"
)

func (w *InRepoWorker) Diff(ctx context.Context, req IdentifyWorkRequest) ([]Work, error) {
	// Cannot diff without both stores
	if w.Tracker.Intent == nil {
		return nil, nil
	}

	var out []Work

	state, intent := w.Tracker.State, w.Tracker.Intent

	prefix := "**"
	if req.GitFilter == GitFilterCurrentRepoOnly {
		prefix = fmt.Sprintf("%s/-/**", w.Tracker.Ref.Repo)
	}

	stateRefs, err := state.Match(ctx, prefix+"/@/{deploy}/*", prefix+"/@*/custom/*", "@/{custom,environment}/*")
	if err != nil {
		return nil, fmt.Errorf("failed to match state refs: %w", err)
	}

	intentRefs, err := intent.Match(ctx, prefix+"/@/{deploy}/*", prefix+"/@*/custom/*", "@/{custom,environment}/*")
	if err != nil {
		return nil, fmt.Errorf("failed to match intent refs: %w", err)
	}

	log.Debug("Diffing", "stateRefs", stateRefs, "intentRefs", intentRefs)

	var (
		stateRefsMap        = make(map[string]struct{})
		intentRefsMap       = make(map[string]struct{})
		stateToIntentRefSet = make(map[string]string)
	)

	for _, ref := range stateRefs {
		stateRefsMap[ref] = struct{}{}
	}

	// Iterate over intent to find any without matching state,
	// and map intent refs to resolved state refs for comparison
	for _, ref := range intentRefs {
		intentRefsMap[ref] = struct{}{}
		ir, err := refs.Parse(ref)
		if err != nil {
			return nil, fmt.Errorf("failed to parse intent ref: %w", err)
		}
		resolvedRef, err := state.ResolveLink(ctx, ir.String())
		if err != nil {
			return nil, fmt.Errorf("failed to resolve intent ref: %w", err)
		}
		_, existsAsIs := stateRefsMap[ref]
		_, existsResolved := stateRefsMap[resolvedRef]

		if !existsAsIs && !existsResolved {
			out = append(out, Work{
				Ref:      ir,
				WorkType: WorkTypeCreate,
			})
		} else if existsResolved {
			stateToIntentRefSet[resolvedRef] = ref
			delete(stateRefsMap, resolvedRef)
		} else {
			delete(stateRefsMap, ref)
		}
	}

	// Iterate over state again to find any without matching intent
	for ref := range stateRefsMap {
		if _, exists := intentRefsMap[ref]; !exists {
			ir, err := refs.Parse(ref)
			if err != nil {
				return nil, fmt.Errorf("failed to parse state ref: %w", err)
			}
			out = append(out, Work{
				Ref:      ir,
				WorkType: WorkTypeDelete,
			})
		}
	}

	log.Debug("After matching", "stateToIntentRefSet", stateToIntentRefSet)

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
			out = append(out, Work{
				Ref:      ir,
				WorkType: WorkTypeUpdate,
			})
		}
	}

	if req.IntentChanges == nil {
		return out, nil
	}

	var resolvedIntentChanges = make(map[string]struct{})
	for ref := range req.IntentChanges {
		resolvedRef, err := state.ResolveLink(ctx, ref)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve intent ref: %w", err)
		}
		resolvedIntentChanges[resolvedRef] = struct{}{}
	}

	var filteredWork []Work
	for _, w := range out {
		if _, exists := resolvedIntentChanges[w.Ref.String()]; exists {
			filteredWork = append(filteredWork, w)
		}
	}
	out = filteredWork

	return out, nil
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
