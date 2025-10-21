package git

import (
	"context"
	"os/exec"
	"strings"
	"testing"
)

func TestRemoteGitCreateBranchOnBareRepo(t *testing.T) {
	ctx := context.Background()

	// Create a bare repository under testdata
	bareRepoPath := "testdata/create-branch-bare-repo.git"

	// Remove if it exists from previous run
	exec.Command("rm", "-rf", bareRepoPath).Run()

	// Initialize bare repository using git command
	err := exec.Command("git", "init", "--bare", bareRepoPath).Run()
	if err != nil {
		t.Fatalf("Failed to create bare repo: %v", err)
	}

	// Connect to the bare repository with a user
	user := &GitUser{
		Name:  "Test Author",
		Email: "author@example.com",
	}

	rg, err := NewRemoteGitWithUser("file://"+bareRepoPath, user)
	if err != nil {
		t.Fatal(err)
	}

	// Create a new branch on the empty bare repo (orphan branch)
	err = rg.CreateBranch(ctx, "main", "", "Initial commit on main branch")
	if err != nil {
		t.Fatalf("CreateBranch failed on bare repo: %v", err)
	}

	// Verify the branch was created
	refs, err := rg.BranchRefs(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if len(refs) != 1 {
		t.Fatalf("Expected 1 ref, got %d", len(refs))
	}

	if refs[0].Name != "refs/heads/main" {
		t.Errorf("Expected ref name 'refs/heads/main', got '%s'", refs[0].Name)
	}

	// Verify the commit exists and has an empty tree
	tree, err := rg.GetTree(ctx, refs[0].Hash)
	if err != nil {
		t.Fatal(err)
	}

	if len(tree.Children) != 0 {
		t.Errorf("Expected empty tree, got %d children", len(tree.Children))
	}

	t.Log("CreateBranch on bare repo test completed successfully")
}

func TestRemoteGitCreateBranchFromExisting(t *testing.T) {
	ctx := context.Background()

	// Create a bare repository under testdata
	bareRepoPath := "testdata/create-branch-from-existing.git"

	// Remove if it exists from previous run
	exec.Command("rm", "-rf", bareRepoPath).Run()

	// Initialize bare repository using git command
	err := exec.Command("git", "init", "--bare", bareRepoPath).Run()
	if err != nil {
		t.Fatalf("Failed to create bare repo: %v", err)
	}

	// Connect to the bare repository with a user
	user := &GitUser{
		Name:  "Test Author",
		Email: "author@example.com",
	}

	rg, err := NewRemoteGitWithUser("file://"+bareRepoPath, user)
	if err != nil {
		t.Fatal(err)
	}

	// Create initial content on main branch
	objectsByPath := map[string]string{
		"README.md":   "# Test Repository\n",
		"src/main.go": "package main\n\nfunc main() {}\n",
	}

	err = rg.Push(ctx, "refs/heads/main", toGitObjects(objectsByPath), "Initial commit")
	if err != nil {
		t.Fatalf("Initial push failed: %v", err)
	}

	// Get the main branch ref
	refs, err := rg.BranchRefs(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if len(refs) != 1 {
		t.Fatalf("Expected 1 ref after initial push, got %d", len(refs))
	}

	mainHash := refs[0].Hash

	// Create a new branch from main using branch name
	err = rg.CreateBranch(ctx, "develop", "main", "Create develop branch")
	if err != nil {
		t.Fatalf("CreateBranch from main failed: %v", err)
	}

	// Verify both branches exist
	refs, err = rg.BranchRefs(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if len(refs) != 2 {
		t.Fatalf("Expected 2 refs, got %d", len(refs))
	}

	// Find develop branch
	var developHash string
	for _, ref := range refs {
		if ref.Name == "refs/heads/develop" {
			developHash = ref.Hash
			break
		}
	}

	if developHash == "" {
		t.Fatal("develop branch not found")
	}

	// Verify develop points to the same commit as main
	if developHash != mainHash {
		t.Errorf("Expected develop to point to same commit as main (%s), got %s", mainHash, developHash)
	}

	// Verify the tree contents are the same
	developTree, err := rg.GetTree(ctx, developHash)
	if err != nil {
		t.Fatal(err)
	}

	// Check files exist
	if _, err := developTree.NodeAtPath("README.md"); err != nil {
		t.Errorf("README.md not found in develop branch: %v", err)
	}

	if _, err := developTree.NodeAtPath("src/main.go"); err != nil {
		t.Errorf("src/main.go not found in develop branch: %v", err)
	}

	t.Log("CreateBranch from existing branch test completed successfully")
}

func TestRemoteGitCreateBranchFromCommitHash(t *testing.T) {
	ctx := context.Background()

	// Create a bare repository under testdata
	bareRepoPath := "testdata/create-branch-from-hash.git"

	// Remove if it exists from previous run
	exec.Command("rm", "-rf", bareRepoPath).Run()

	// Initialize bare repository using git command
	err := exec.Command("git", "init", "--bare", bareRepoPath).Run()
	if err != nil {
		t.Fatalf("Failed to create bare repo: %v", err)
	}

	// Connect to the bare repository with a user
	user := &GitUser{
		Name:  "Test Author",
		Email: "author@example.com",
	}

	rg, err := NewRemoteGitWithUser("file://"+bareRepoPath, user)
	if err != nil {
		t.Fatal(err)
	}

	// Create initial content
	objectsByPath := map[string]string{
		"file1.txt": "Version 1\n",
	}

	err = rg.Push(ctx, "refs/heads/main", toGitObjects(objectsByPath), "First commit")
	if err != nil {
		t.Fatalf("First push failed: %v", err)
	}

	// Get the first commit hash
	refs, err := rg.BranchRefs(ctx)
	if err != nil {
		t.Fatal(err)
	}
	firstCommitHash := refs[0].Hash

	// Make a second commit
	objectsByPath["file2.txt"] = "Version 2\n"
	err = rg.Push(ctx, "refs/heads/main", toGitObjects(objectsByPath), "Second commit")
	if err != nil {
		t.Fatalf("Second push failed: %v", err)
	}

	// Create a branch from the first commit hash
	err = rg.CreateBranch(ctx, "release-v1", firstCommitHash, "Create release branch from v1")
	if err != nil {
		t.Fatalf("CreateBranch from commit hash failed: %v", err)
	}

	// Verify the branch exists
	refs, err = rg.BranchRefs(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if len(refs) != 2 {
		t.Fatalf("Expected 2 refs, got %d", len(refs))
	}

	// Find release-v1 branch
	var releaseHash string
	for _, ref := range refs {
		if ref.Name == "refs/heads/release-v1" {
			releaseHash = ref.Hash
			break
		}
	}

	if releaseHash == "" {
		t.Fatal("release-v1 branch not found")
	}

	// Verify release-v1 points to the first commit
	if releaseHash != firstCommitHash {
		t.Errorf("Expected release-v1 to point to first commit (%s), got %s", firstCommitHash, releaseHash)
	}

	// Verify the tree only has file1.txt
	releaseTree, err := rg.GetTree(ctx, releaseHash)
	if err != nil {
		t.Fatal(err)
	}

	if node, err := releaseTree.NodeAtPath("file1.txt"); node == nil || err != nil {
		t.Errorf("file1.txt not found in release-v1 branch: %v", err)
	}

	if node, err := releaseTree.NodeAtPath("file2.txt"); node != nil || err != nil {
		t.Error("file2.txt should not exist in release-v1 branch")
	}

	t.Log("CreateBranch from commit hash test completed successfully")
}

func TestRemoteGitCreateBranchErrors(t *testing.T) {
	ctx := context.Background()

	// Create a bare repository under testdata
	bareRepoPath := "testdata/create-branch-errors.git"

	// Remove if it exists from previous run
	exec.Command("rm", "-rf", bareRepoPath).Run()

	// Initialize bare repository using git command
	err := exec.Command("git", "init", "--bare", bareRepoPath).Run()
	if err != nil {
		t.Fatalf("Failed to create bare repo: %v", err)
	}

	// Test without user
	rgNoUser, err := NewRemoteGitWithUser("file://"+bareRepoPath, nil)
	if err != nil {
		t.Fatal(err)
	}

	err = rgNoUser.CreateBranch(ctx, "test", "", "Test commit")
	if err == nil {
		t.Error("Expected error for nil user, got nil")
	}
	if err != nil && !strings.Contains(err.Error(), "user must be set") {
		t.Errorf("Expected 'user must be set' error, got: %v", err)
	}

	// Test with invalid user (missing name)
	rgInvalidUser, err := NewRemoteGitWithUser("file://"+bareRepoPath, &GitUser{
		Email: "test@example.com",
	})
	if err != nil {
		t.Fatal(err)
	}

	err = rgInvalidUser.CreateBranch(ctx, "test", "", "Test commit")
	if err == nil {
		t.Error("Expected error for invalid user, got nil")
	}

	// Create a valid user for remaining tests
	user := &GitUser{
		Name:  "Test Author",
		Email: "author@example.com",
	}

	rg, err := NewRemoteGitWithUser("file://"+bareRepoPath, user)
	if err != nil {
		t.Fatal(err)
	}

	// Create a branch
	err = rg.CreateBranch(ctx, "main", "", "Initial commit")
	if err != nil {
		t.Fatalf("CreateBranch failed: %v", err)
	}

	// Try to create the same branch again
	err = rg.CreateBranch(ctx, "main", "", "Duplicate commit")
	if err == nil {
		t.Error("Expected error for duplicate branch, got nil")
	}
	if err != nil && !strings.Contains(err.Error(), "already exists") {
		t.Errorf("Expected 'already exists' error, got: %v", err)
	}

	// Try to create a branch from non-existent source
	err = rg.CreateBranch(ctx, "feature", "nonexistent", "Test commit")
	if err == nil {
		t.Error("Expected error for non-existent source ref, got nil")
	}
	if err != nil && !strings.Contains(err.Error(), "not found") {
		t.Errorf("Expected 'not found' error, got: %v", err)
	}

	t.Log("CreateBranch error handling test completed successfully")
}

func TestRemoteGitCreateBranchWithRefsHeadsPrefix(t *testing.T) {
	ctx := context.Background()

	// Create a bare repository under testdata
	bareRepoPath := "testdata/create-branch-refs-prefix.git"

	// Remove if it exists from previous run
	exec.Command("rm", "-rf", bareRepoPath).Run()

	// Initialize bare repository using git command
	err := exec.Command("git", "init", "--bare", bareRepoPath).Run()
	if err != nil {
		t.Fatalf("Failed to create bare repo: %v", err)
	}

	user := &GitUser{
		Name:  "Test Author",
		Email: "author@example.com",
	}

	rg, err := NewRemoteGitWithUser("file://"+bareRepoPath, user)
	if err != nil {
		t.Fatal(err)
	}

	// Create a branch with refs/heads/ prefix
	err = rg.CreateBranch(ctx, "refs/heads/main", "", "Initial commit")
	if err != nil {
		t.Fatalf("CreateBranch with refs/heads/ prefix failed: %v", err)
	}

	// Verify the branch was created
	refs, err := rg.BranchRefs(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if len(refs) != 1 {
		t.Fatalf("Expected 1 ref, got %d", len(refs))
	}

	if refs[0].Name != "refs/heads/main" {
		t.Errorf("Expected ref name 'refs/heads/main', got '%s'", refs[0].Name)
	}

	// Create another branch without the prefix
	err = rg.CreateBranch(ctx, "develop", "", "Create develop")
	if err != nil {
		t.Fatalf("CreateBranch without prefix failed: %v", err)
	}

	// Verify both branches exist
	refs, err = rg.BranchRefs(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if len(refs) != 2 {
		t.Fatalf("Expected 2 refs, got %d", len(refs))
	}

	t.Log("CreateBranch with refs/heads/ prefix test completed successfully")
}
