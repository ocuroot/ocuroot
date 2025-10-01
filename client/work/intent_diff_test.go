package work

import (
	"context"
	"os"
	"sort"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/ocuroot/ocuroot/client/release"
	"github.com/ocuroot/ocuroot/refs"
	"github.com/ocuroot/ocuroot/refs/refstore"
)

func TestIntentDiffCustomRefs(t *testing.T) {
	// Clean up testdata directories
	os.RemoveAll("./testdata/state")
	os.RemoveAll("./testdata/intent")
	os.MkdirAll("./testdata/state", 0755)
	os.MkdirAll("./testdata/intent", 0755)

	stateStore, err := refstore.NewFSRefStore("./testdata/state")
	if err != nil {
		t.Fatal(err)
	}
	intentStore, err := refstore.NewFSRefStore("./testdata/intent")
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()

	// Create some state data using proper ref format with custom subpath type
	err = stateStore.Set(ctx, "github.com/example/repo/-/myapp/@/custom/image-tag", map[string]interface{}{
		"tag":    "v1.0.0",
		"digest": "sha256:abc123",
	})
	if err != nil {
		t.Fatal(err)
	}

	err = stateStore.Set(ctx, "github.com/example/repo/-/database/@/custom/connection", map[string]interface{}{
		"host": "db.example.com",
		"port": 5432,
	})
	if err != nil {
		t.Fatal(err)
	}

	// Add items that will be deleted (exist in state but not in intent)
	err = stateStore.Set(ctx, "github.com/example/repo/-/oldservice/@/custom/metadata", map[string]interface{}{
		"version": "0.9.0",
		"status":  "deprecated",
	})
	if err != nil {
		t.Fatal(err)
	}

	err = stateStore.Set(ctx, "@/custom/cluster-info", map[string]interface{}{
		"region":    "us-west-2",
		"nodeCount": 5,
	})
	if err != nil {
		t.Fatal(err)
	}

	// Create intent data that includes items not in state (should result in "create" work)
	// First, copy existing state items
	err = intentStore.Set(ctx, "github.com/example/repo/-/myapp/@/custom/image-tag", map[string]interface{}{
		"tag":    "v1.0.0",
		"digest": "sha256:abc123",
	})
	if err != nil {
		t.Fatal(err)
	}

	// Update database connection with different values (should result in "update" work)
	err = intentStore.Set(ctx, "github.com/example/repo/-/database/@/custom/connection", map[string]interface{}{
		"host": "db-new.example.com",
		"port": 5433,
	})
	if err != nil {
		t.Fatal(err)
	}

	// Add new items that don't exist in state (these should generate "create" work)
	err = intentStore.Set(ctx, "github.com/example/repo/-/newservice/@/custom/build-info", map[string]interface{}{
		"commit": "abc123def",
		"branch": "main",
	})
	if err != nil {
		t.Fatal(err)
	}

	err = intentStore.Set(ctx, "github.com/example/repo/-/cache/@/custom/settings", map[string]interface{}{
		"ttl":     3600,
		"maxSize": "100MB",
	})
	if err != nil {
		t.Fatal(err)
	}

	err = intentStore.Set(ctx, "@/custom/feature-flags", map[string]interface{}{
		"enableNewUI": true,
		"debugMode":   false,
	})
	if err != nil {
		t.Fatal(err)
	}

	// Create worker and run diff
	worker := &Worker{
		Tracker: release.TrackerConfig{
			State:  stateStore,
			Intent: intentStore,
		},
	}

	work, err := worker.Diff(ctx, IdentifyWorkRequest{})
	if err != nil {
		t.Fatal(err)
	}

	// Verify we got the expected work items of all types
	expectedWork := []Work{
		// Create work items (exist in intent but not in state)
		{
			Ref:      mustParseRef("github.com/example/repo/-/newservice/@/custom/build-info"),
			WorkType: WorkTypeCreate,
		},
		{
			Ref:      mustParseRef("github.com/example/repo/-/cache/@/custom/settings"),
			WorkType: WorkTypeCreate,
		},
		{
			Ref:      mustParseRef("@/custom/feature-flags"),
			WorkType: WorkTypeCreate,
		},
		// Update work items (exist in both but with different content)
		{
			Ref:      mustParseRef("github.com/example/repo/-/database/@/custom/connection"),
			WorkType: WorkTypeUpdate,
		},
		// Delete work items (exist in state but not in intent)
		{
			Ref:      mustParseRef("github.com/example/repo/-/oldservice/@/custom/metadata"),
			WorkType: WorkTypeDelete,
		},
		{
			Ref:      mustParseRef("@/custom/cluster-info"),
			WorkType: WorkTypeDelete,
		},
	}

	// Sort both slices by ref string for consistent comparison
	sort.Slice(work, func(i, j int) bool {
		return work[i].Ref.String() < work[j].Ref.String()
	})
	sort.Slice(expectedWork, func(i, j int) bool {
		return expectedWork[i].Ref.String() < expectedWork[j].Ref.String()
	})

	if diff := cmp.Diff(expectedWork, work, cmpopts.IgnoreUnexported(refs.Ref{})); diff != "" {
		t.Errorf("Work items mismatch (-want +got):\n%s", diff)
	}

	// Log summary of work types found
	workTypeCounts := make(map[WorkType]int)
	for _, w := range work {
		workTypeCounts[w.WorkType]++
	}
	t.Logf("Found work items: %d create, %d update, %d delete",
		workTypeCounts[WorkTypeCreate],
		workTypeCounts[WorkTypeUpdate],
		workTypeCounts[WorkTypeDelete])
}

func TestIntentDiffRepoFiltering(t *testing.T) {
	// Clean up testdata directories
	os.RemoveAll("./testdata/repo_filter_state")
	os.RemoveAll("./testdata/repo_filter_intent")
	os.MkdirAll("./testdata/repo_filter_state", 0755)
	os.MkdirAll("./testdata/repo_filter_intent", 0755)

	stateStore, err := refstore.NewFSRefStore("./testdata/repo_filter_state")
	if err != nil {
		t.Fatal(err)
	}
	intentStore, err := refstore.NewFSRefStore("./testdata/repo_filter_intent")
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()

	// Create state data for MULTIPLE repos to test filtering
	// Target repo: github.com/example/repo
	err = stateStore.Set(ctx, "github.com/example/repo/-/myapp/@/custom/image-tag", map[string]interface{}{
		"tag":    "v1.0.0",
		"digest": "sha256:abc123",
	})
	if err != nil {
		t.Fatal(err)
	}

	err = stateStore.Set(ctx, "github.com/example/repo/-/database/@/custom/connection", map[string]interface{}{
		"host": "db.example.com",
		"port": 5432,
	})
	if err != nil {
		t.Fatal(err)
	}

	// Different repo: github.com/other/repo (should be filtered out)
	err = stateStore.Set(ctx, "github.com/other/repo/-/service/@/custom/config", map[string]interface{}{
		"env":      "production",
		"replicas": 3,
	})
	if err != nil {
		t.Fatal(err)
	}

	err = stateStore.Set(ctx, "github.com/other/repo/-/cache/@/custom/settings", map[string]interface{}{
		"ttl":  3600,
		"size": "1GB",
	})
	if err != nil {
		t.Fatal(err)
	}

	// Global refs (no repo prefix) - should always be included
	err = stateStore.Set(ctx, "@/custom/cluster-info", map[string]interface{}{
		"region":    "us-west-2",
		"nodeCount": 5,
	})
	if err != nil {
		t.Fatal(err)
	}

	// Create intent data - mix of target repo, other repo, and global
	// Target repo changes
	err = intentStore.Set(ctx, "github.com/example/repo/-/myapp/@/custom/image-tag", map[string]interface{}{
		"tag":    "v1.1.0", // Updated version
		"digest": "sha256:def456",
	})
	if err != nil {
		t.Fatal(err)
	}

	err = intentStore.Set(ctx, "github.com/example/repo/-/newservice/@/custom/build-info", map[string]interface{}{
		"commit": "abc123def",
		"branch": "main",
	})
	if err != nil {
		t.Fatal(err)
	}

	// Other repo changes (should be filtered out when filtering is enabled)
	err = intentStore.Set(ctx, "github.com/other/repo/-/service/@/custom/config", map[string]interface{}{
		"env":      "staging", // Updated environment
		"replicas": 2,
	})
	if err != nil {
		t.Fatal(err)
	}

	err = intentStore.Set(ctx, "github.com/other/repo/-/monitoring/@/custom/alerts", map[string]interface{}{
		"enabled":   true,
		"threshold": 0.8,
	})
	if err != nil {
		t.Fatal(err)
	}

	// Global changes (should always be included)
	err = intentStore.Set(ctx, "@/custom/cluster-info", map[string]interface{}{
		"region":    "us-east-1", // Updated region
		"nodeCount": 3,
	})
	if err != nil {
		t.Fatal(err)
	}

	// Test 1: No filtering (should include all repos)
	worker := &Worker{
		Tracker: release.TrackerConfig{
			State:  stateStore,
			Intent: intentStore,
		},
	}

	allWork, err := worker.Diff(ctx, IdentifyWorkRequest{GitFilter: GitFilterNone})
	if err != nil {
		t.Fatal(err)
	}

	// Test 2: Filter to current repo only
	worker.Tracker.Ref = mustParseRef("github.com/example/repo/-/release1")

	filteredWork, err := worker.Diff(ctx, IdentifyWorkRequest{GitFilter: GitFilterCurrentRepoOnly})
	if err != nil {
		t.Fatal(err)
	}

	// Verify filtering worked correctly
	t.Logf("No filtering: found %d work items", len(allWork))
	t.Logf("Repo filtering: found %d work items", len(filteredWork))

	// Count work items by repo
	allWorkByRepo := make(map[string]int)
	filteredWorkByRepo := make(map[string]int)

	for _, work := range allWork {
		repo := work.Ref.Repo
		if repo == "" {
			repo = "global"
		}
		allWorkByRepo[repo]++
	}

	for _, work := range filteredWork {
		repo := work.Ref.Repo
		if repo == "" {
			repo = "global"
		}
		filteredWorkByRepo[repo]++
	}

	t.Logf("All work by repo: %+v", allWorkByRepo)
	t.Logf("Filtered work by repo: %+v", filteredWorkByRepo)

	// Verify that filtering worked
	if len(filteredWork) >= len(allWork) {
		t.Error("Filtered work should have fewer items than unfiltered work")
	}

	// Verify that filtered work only contains target repo and global items
	for _, work := range filteredWork {
		repo := work.Ref.Repo
		if repo != "" && repo != "github.com/example/repo" {
			t.Errorf("Filtered work contains item from wrong repo: %s (ref: %s)", repo, work.Ref.String())
		}
	}

	// Verify that target repo items are still present
	targetRepoCount := filteredWorkByRepo["github.com/example/repo"]
	if targetRepoCount == 0 {
		t.Error("Filtered work should contain items from target repo")
	}

	// Verify that global items are still present
	globalCount := filteredWorkByRepo["global"]
	if globalCount == 0 {
		t.Error("Filtered work should contain global items")
	}

	// Verify that other repo items are excluded
	otherRepoCount := filteredWorkByRepo["github.com/other/repo"]
	if otherRepoCount > 0 {
		t.Error("Filtered work should not contain items from other repos")
	}

	t.Logf("Repo filtering test completed successfully")
}
