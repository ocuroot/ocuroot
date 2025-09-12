package work

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/ocuroot/ocuroot/client/release"
	librelease "github.com/ocuroot/ocuroot/lib/release"
	"github.com/ocuroot/ocuroot/refs"
	"github.com/ocuroot/ocuroot/refs/refstore"
	"github.com/ocuroot/ocuroot/sdk"
	"github.com/ocuroot/ocuroot/store/models"
)

func TestReadyRuns(t *testing.T) {
	// Clean up and create test directories
	os.RemoveAll("./testdata/ready_runs_state")
	os.MkdirAll("./testdata/ready_runs_state", 0755)

	stateStore, err := refstore.NewFSRefStore("./testdata/ready_runs_state")
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()

	// Create test data for different scenarios
	releaseRef := mustParseRef("github.com/example/repo/-/release1")
	releaseWithCommitRef := mustParseRef("github.com/example/repo/-/release1/@abc123")
	
	// Create release information in state store
	releaseInfo := librelease.ReleaseInfo{
		Commit: "abc123",
	}
	err = stateStore.Set(ctx, releaseWithCommitRef.String(), releaseInfo)
	if err != nil {
		t.Fatal(err)
	}

	// Scenario 1: Ready run with pending status and all inputs satisfied
	readyRunRef := mustParseRef("github.com/example/repo/-/release1/@abc123/deploy/production/1")
	readyRun := models.Run{
		Type:    models.RunTypeUp,
		Release: releaseRef,
		Functions: []*models.Function{
			{
				Fn: sdk.FunctionDef{
					Name: "deploy",
				},
				Dependencies: []refs.Ref{}, // No dependencies
				Inputs: map[string]sdk.InputDescriptor{
					"image_tag": {
						Value: "v1.2.3", // Input has value
					},
					"config": {
						Value: "default-config", // Input has value
					},
				},
			},
		},
	}
	err = stateStore.Set(ctx, readyRunRef.String(), readyRun)
	if err != nil {
		t.Fatal(err)
	}
	err = stateStore.Set(ctx, readyRunRef.String()+"/status/pending", "pending")
	if err != nil {
		t.Fatal(err)
	}

	// Scenario 2: Ready run with paused status
	pausedRunRef := mustParseRef("github.com/example/repo/-/release1/@abc123/task/backup/1")
	pausedRun := models.Run{
		Type:    models.RunTypeTask,
		Release: releaseRef,
		Functions: []*models.Function{
			{
				Fn: sdk.FunctionDef{
					Name: "backup",
				},
				Dependencies: []refs.Ref{}, // No dependencies
				Inputs: map[string]sdk.InputDescriptor{
					"database_url": {
						Value: "postgres://localhost:5432/db",
					},
				},
			},
		},
	}
	err = stateStore.Set(ctx, pausedRunRef.String(), pausedRun)
	if err != nil {
		t.Fatal(err)
	}
	err = stateStore.Set(ctx, pausedRunRef.String()+"/status/paused", "paused")
	if err != nil {
		t.Fatal(err)
	}

	// Scenario 3: Run waiting on inputs - some inputs missing (should not be ready)
	waitingRunRef := mustParseRef("github.com/example/repo/-/release1/@abc123/deploy/staging/1")
	waitingRun := models.Run{
		Type:    models.RunTypeUp,
		Release: releaseRef,
		Functions: []*models.Function{
			{
				Fn: sdk.FunctionDef{
					Name: "deploy_staging",
				},
				Dependencies: []refs.Ref{}, // No dependencies
				Inputs: map[string]sdk.InputDescriptor{
					"image_tag": {
						Value: "v1.2.3", // This input exists
					},
					"config": {
						Value: "staging-config", // This input has value
					},
					"secret_key": {
						Ref: mustParseRefPtr("github.com/example/repo/-/release1/@abc123/deploy/missing-secret/1"), // Reference to non-existent input
					},
				},
			},
		},
	}
	err = stateStore.Set(ctx, waitingRunRef.String(), waitingRun)
	if err != nil {
		t.Fatal(err)
	}
	err = stateStore.Set(ctx, waitingRunRef.String()+"/status/pending", "pending")
	if err != nil {
		t.Fatal(err)
	}

	// Scenario 4: Run with dependencies that are satisfied
	dependentRunRef := mustParseRef("github.com/example/repo/-/release1/@abc123/deploy/integration/1")
	// Create the dependency run first
	depRunRef := mustParseRef("github.com/example/repo/-/release1/@abc123/deploy/database/1")
	depRun := models.Run{
		Type:    models.RunTypeUp,
		Release: releaseRef,
		Functions: []*models.Function{
			{
				Fn: sdk.FunctionDef{Name: "setup_db"},
				Inputs: map[string]sdk.InputDescriptor{
					"db_name": {Value: "testdb"},
				},
			},
		},
	}
	err = stateStore.Set(ctx, depRunRef.String(), depRun)
	if err != nil {
		t.Fatal(err)
	}
	err = stateStore.Set(ctx, depRunRef.String()+"/status/success", "success")
	if err != nil {
		t.Fatal(err)
	}

	dependentRun := models.Run{
		Type:    models.RunTypeUp,
		Release: releaseRef,
		Functions: []*models.Function{
			{
				Fn: sdk.FunctionDef{
					Name: "deploy_integration",
				},
				Dependencies: []refs.Ref{depRunRef}, // Depends on database setup
				Inputs: map[string]sdk.InputDescriptor{
					"app_version": {
						Value: "v2.0.0",
					},
				},
			},
		},
	}
	err = stateStore.Set(ctx, dependentRunRef.String(), dependentRun)
	if err != nil {
		t.Fatal(err)
	}
	err = stateStore.Set(ctx, dependentRunRef.String()+"/status/pending", "pending")
	if err != nil {
		t.Fatal(err)
	}

	// Scenario 5: Run with unsatisfied dependencies (should not be ready)
	blockedRunRef := mustParseRef("github.com/example/repo/-/release1/@abc123/task/cleanup/1")
	// Create an incomplete dependency - this dependency should NOT satisfy the dependent run
	// We'll create a dependency that doesn't exist at all, making it clearly unsatisfied
	incompleteDepRef := mustParseRef("github.com/example/repo/-/release1/@abc123/task/missing-validation/1")

	blockedRun := models.Run{
		Type:    models.RunTypeTask,
		Release: releaseRef,
		Functions: []*models.Function{
			{
				Fn: sdk.FunctionDef{
					Name: "cleanup",
				},
				Dependencies: []refs.Ref{incompleteDepRef}, // Depends on incomplete validation
				Inputs: map[string]sdk.InputDescriptor{
					"force": {
						Value: false,
					},
				},
			},
		},
	}
	err = stateStore.Set(ctx, blockedRunRef.String(), blockedRun)
	if err != nil {
		t.Fatal(err)
	}
	err = stateStore.Set(ctx, blockedRunRef.String()+"/status/pending", "pending")
	if err != nil {
		t.Fatal(err)
	}

	// Create worker and test ReadyRuns
	worker := &Worker{
		Tracker: release.TrackerConfig{
			State: stateStore,
		},
	}

	readyWork, err := worker.ReadyRuns(ctx, IndentifyWorkRequest{})
	if err != nil {
		t.Fatal(err)
	}

	// Log all found runs for debugging
	t.Logf("Found %d runs:", len(readyWork))
	for _, work := range readyWork {
		t.Logf("  - %s (type: %s)", work.Ref.String(), work.WorkType)
	}

	// Verify that all returned work items have the correct type
	for _, work := range readyWork {
		if work.WorkType != WorkTypeRun {
			t.Errorf("Expected WorkType to be %s, got %s", WorkTypeRun, work.WorkType)
		}
	}

	// Categorize the found runs for analysis
	foundRefs := make(map[string]bool)
	for _, work := range readyWork {
		foundRefs[work.Ref.String()] = true
	}

	// Test scenarios:
	scenarios := map[string]struct {
		ref           string
		shouldBeReady bool
		description   string
	}{
		"ready_pending": {
			ref:           readyRunRef.String(),
			shouldBeReady: true,
			description:   "Run with pending status and all inputs satisfied",
		},
		"ready_paused": {
			ref:           pausedRunRef.String(),
			shouldBeReady: true,
			description:   "Run with paused status and all inputs satisfied",
		},
		"waiting_on_input": {
			ref:           waitingRunRef.String(),
			shouldBeReady: false,
			description:   "Run waiting on missing input (should not be ready)",
		},
		"satisfied_dependency": {
			ref:           dependentRunRef.String(),
			shouldBeReady: true, // Dependencies are satisfied
			description:   "Run with satisfied dependencies",
		},
		"unsatisfied_dependency": {
			ref:           blockedRunRef.String(),
			shouldBeReady: false, // Dependency is not complete
			description:   "Run with unsatisfied dependency (should not be ready)",
		},
	}

	// Verify each scenario
	for name, scenario := range scenarios {
		found := foundRefs[scenario.ref]
		if scenario.shouldBeReady && !found {
			t.Errorf("Scenario %s: %s - expected to be ready but not found", name, scenario.description)
		} else if !scenario.shouldBeReady && found {
			t.Errorf("Scenario %s: %s - should not be ready but was found", name, scenario.description)
		} else {
			t.Logf("✓ Scenario %s: %s - behaved as expected", name, scenario.description)
		}
	}

	// Summary
	readyCount := 0
	for _, scenario := range scenarios {
		if scenario.shouldBeReady {
			readyCount++
		}
	}

	t.Logf("Test completed: found %d runs, expected %d to be ready", len(readyWork), readyCount)
}

func TestReadyRunsRepoFiltering(t *testing.T) {
	// Clean up and create test directories
	os.RemoveAll("./testdata/ready_runs_repo_filter")
	os.MkdirAll("./testdata/ready_runs_repo_filter", 0755)

	stateStore, err := refstore.NewFSRefStore("./testdata/ready_runs_repo_filter")
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()

	// Create runs for multiple repos to test filtering
	targetRepo := "github.com/example/repo"
	otherRepo := "github.com/other/repo"
	
	// Create release information for both repos
	targetReleaseRef := mustParseRef(targetRepo + "/-/release1")
	targetReleaseWithCommitRef := mustParseRef(targetRepo + "/-/release1/@abc123")
	targetReleaseInfo := librelease.ReleaseInfo{
		Commit: "abc123",
	}
	err = stateStore.Set(ctx, targetReleaseWithCommitRef.String(), targetReleaseInfo)
	if err != nil {
		t.Fatal(err)
	}
	
	otherReleaseRef := mustParseRef(otherRepo + "/-/release1")
	otherReleaseWithCommitRef := mustParseRef(otherRepo + "/-/release1/@def456")
	otherReleaseInfo := librelease.ReleaseInfo{
		Commit: "def456",
	}
	err = stateStore.Set(ctx, otherReleaseWithCommitRef.String(), otherReleaseInfo)
	if err != nil {
		t.Fatal(err)
	}
	
	// Target repo runs (should be included)
	targetRun1 := mustParseRef(targetRepo + "/-/release1/@abc123/deploy/production/1")
	targetRun2 := mustParseRef(targetRepo + "/-/release1/@abc123/task/backup/1")
	
	// Other repo runs (should be filtered out)
	otherRun1 := mustParseRef(otherRepo + "/-/release1/@def456/deploy/staging/1")
	otherRun2 := mustParseRef(otherRepo + "/-/release1/@def456/task/cleanup/1")

	// Create run data for all runs
	runs := []struct {
		ref    refs.Ref
		status string
	}{
		{targetRun1, "pending"},
		{targetRun2, "paused"},
		{otherRun1, "pending"},
		{otherRun2, "pending"},
	}

	for _, run := range runs {
		var releaseRef refs.Ref
		if run.ref.Repo == targetRepo {
			releaseRef = targetReleaseRef
		} else {
			releaseRef = otherReleaseRef
		}
		
		runData := models.Run{
			Type:    models.RunTypeUp,
			Release: releaseRef,
			Functions: []*models.Function{
				{
					Fn: sdk.FunctionDef{Name: "test_function"},
					Inputs: map[string]sdk.InputDescriptor{
						"input1": {Value: "test_value"},
					},
				},
			},
		}
		
		err = stateStore.Set(ctx, run.ref.String(), runData)
		if err != nil {
			t.Fatal(err)
		}
		
		err = stateStore.Set(ctx, run.ref.String()+"/status/"+run.status, run.status)
		if err != nil {
			t.Fatal(err)
		}
	}

	// Test without filtering (should find all runs)
	worker := &Worker{
		Tracker: release.TrackerConfig{
			State: stateStore,
		},
	}

	allRuns, err := worker.ReadyRuns(ctx, IndentifyWorkRequest{GitFilter: GitFilterNone})
	if err != nil {
		t.Fatal(err)
	}

	// Test with repo filtering
	worker.Tracker.Ref = mustParseRef(targetRepo + "/-/release1")
	
	filteredRuns, err := worker.ReadyRuns(ctx, IndentifyWorkRequest{GitFilter: GitFilterCurrentRepoOnly})
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("No filtering: found %d runs", len(allRuns))
	t.Logf("Repo filtering: found %d runs", len(filteredRuns))

	// Verify filtering worked
	if len(filteredRuns) >= len(allRuns) {
		t.Error("Filtered runs should be fewer than unfiltered runs")
	}

	// Count runs by repo
	filteredByRepo := make(map[string]int)
	for _, work := range filteredRuns {
		filteredByRepo[work.Ref.Repo]++
	}

	// Should only have target repo runs
	if filteredByRepo[targetRepo] == 0 {
		t.Error("Should have runs from target repo")
	}
	if filteredByRepo[otherRepo] > 0 {
		t.Error("Should not have runs from other repo")
	}

	t.Logf("Repo filtering test completed: target repo runs = %d, other repo runs = %d", 
		filteredByRepo[targetRepo], filteredByRepo[otherRepo])
}

func TestReadyRunsCommitFiltering(t *testing.T) {
	// Clean up and create test directories
	os.RemoveAll("./testdata/ready_runs_commit_filter")
	os.MkdirAll("./testdata/ready_runs_commit_filter", 0755)

	stateStore, err := refstore.NewFSRefStore("./testdata/ready_runs_commit_filter")
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()

	targetRepo := "github.com/example/repo"
	otherRepo := "github.com/other/repo"
	targetCommit := "abc123"
	otherCommit := "def456"

	// Create runs for different repos and commits
	runs := []struct {
		ref    refs.Ref
		commit string
		status string
	}{
		// Target repo, target commit (should be included)
		{mustParseRef(targetRepo + "/-/release1/@" + targetCommit + "/deploy/production/1"), targetCommit, "pending"},
		{mustParseRef(targetRepo + "/-/release1/@" + targetCommit + "/task/backup/1"), targetCommit, "paused"},
		
		// Target repo, different commit (should be filtered out)
		{mustParseRef(targetRepo + "/-/release2/@" + otherCommit + "/deploy/staging/1"), otherCommit, "pending"},
		
		// Other repo, target commit (should be filtered out - repo filtering)
		{mustParseRef(otherRepo + "/-/release1/@" + targetCommit + "/deploy/test/1"), targetCommit, "pending"},
		
		// Other repo, other commit (should be filtered out)
		{mustParseRef(otherRepo + "/-/release2/@" + otherCommit + "/task/cleanup/1"), otherCommit, "pending"},
	}

	for _, run := range runs {
		// Create release info for commit filtering
		releaseRef := run.ref.SetSubPath("").SetSubPathType(refs.SubPathTypeNone).SetFragment("")
		releaseInfo := librelease.ReleaseInfo{
			Commit: run.commit,
		}
		err = stateStore.Set(ctx, releaseRef.String(), releaseInfo)
		if err != nil {
			t.Fatal(err)
		}

		// Create run data
		runData := models.Run{
			Type:    models.RunTypeUp,
			Release: releaseRef,
			Functions: []*models.Function{
				{
					Fn: sdk.FunctionDef{Name: "test_function"},
					Inputs: map[string]sdk.InputDescriptor{
						"input1": {Value: "test_value"},
					},
				},
			},
		}
		
		err = stateStore.Set(ctx, run.ref.String(), runData)
		if err != nil {
			t.Fatal(err)
		}
		
		err = stateStore.Set(ctx, run.ref.String()+"/status/"+run.status, run.status)
		if err != nil {
			t.Fatal(err)
		}
	}

	worker := &Worker{
		Tracker: release.TrackerConfig{
			State:  stateStore,
			Ref:    mustParseRef(targetRepo + "/-/release1"),
			Commit: targetCommit,
		},
	}

	// Test without filtering
	allRuns, err := worker.ReadyRuns(ctx, IndentifyWorkRequest{GitFilter: GitFilterNone})
	if err != nil {
		t.Fatal(err)
	}

	// Test with repo filtering only
	repoFilteredRuns, err := worker.ReadyRuns(ctx, IndentifyWorkRequest{GitFilter: GitFilterCurrentRepoOnly})
	if err != nil {
		t.Fatal(err)
	}

	// Test with commit filtering (should imply repo filtering)
	commitFilteredRuns, err := worker.ReadyRuns(ctx, IndentifyWorkRequest{GitFilter: GitFilterCurrentCommitOnly})
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("No filtering: found %d runs", len(allRuns))
	t.Logf("Repo filtering: found %d runs", len(repoFilteredRuns))
	t.Logf("Commit filtering: found %d runs", len(commitFilteredRuns))

	// Verify commit filtering is most restrictive
	if len(commitFilteredRuns) > len(repoFilteredRuns) {
		t.Error("Commit filtering should be more restrictive than repo filtering")
	}
	if len(repoFilteredRuns) > len(allRuns) {
		t.Error("Repo filtering should be more restrictive than no filtering")
	}

	// Verify commit filtering only includes target repo + target commit
	for _, work := range commitFilteredRuns {
		if work.Ref.Repo != targetRepo {
			t.Errorf("Commit filtered run has wrong repo: %s", work.Ref.Repo)
		}
		// The commit is embedded in the ref path as @commit
		if !strings.Contains(work.Ref.String(), "@"+targetCommit) {
			t.Errorf("Commit filtered run has wrong commit in ref: %s", work.Ref.String())
		}
	}

	// Count expected runs (target repo + target commit)
	expectedCommitRuns := 2 // production/1 and backup/1
	if len(commitFilteredRuns) != expectedCommitRuns {
		t.Errorf("Expected %d commit-filtered runs, got %d", expectedCommitRuns, len(commitFilteredRuns))
	}

	t.Logf("Commit filtering test completed successfully")
}

func mustParseRefPtr(refStr string) *refs.Ref {
	ref := mustParseRef(refStr)
	return &ref
}
