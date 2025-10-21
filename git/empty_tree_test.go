package git

import (
	"context"
	"os/exec"
	"testing"
)

func TestRemoteGitPushEmptyTree(t *testing.T) {
	ctx := context.Background()

	// Create a bare repository
	bareRepoPath := "testdata/empty-tree-test-repo.git"
	exec.Command("rm", "-rf", bareRepoPath).Run()

	err := exec.Command("git", "init", "--bare", bareRepoPath).Run()
	if err != nil {
		t.Fatalf("Failed to create bare repo: %v", err)
	}

	// Connect with a user
	user := &GitUser{
		Name:  "Test Author",
		Email: "author@example.com",
	}

	rg, err := NewRemoteGitWithUser("file://"+bareRepoPath, user)
	if err != nil {
		t.Fatal(err)
	}

	// Initial commit with one file
	initialObjects := toGitObjects(map[string]string{
		"file1.txt": "Content 1\n",
	})

	err = rg.Push(ctx, "refs/heads/main", initialObjects, "Initial commit")
	if err != nil {
		t.Fatalf("Initial push failed: %v", err)
	}

	// Second commit: delete the only file (resulting in empty tree)
	deleteObjects := map[string]GitObject{
		"file1.txt": {
			Tombstone: true,
		},
	}

	err = rg.Push(ctx, "refs/heads/main", deleteObjects, "Delete all files")
	if err != nil {
		t.Fatalf("Delete push failed: %v", err)
	}

	// Verify the tree is empty
	refs, err := rg.BranchRefs(ctx)
	if err != nil {
		t.Fatal(err)
	}

	tree, err := rg.GetTree(ctx, refs[0].Hash)
	if err != nil {
		t.Fatal(err)
	}

	paths := tree.Paths()
	if len(paths) != 0 {
		t.Errorf("Expected empty tree, but got %d paths: %v", len(paths), paths)
	}

	t.Log("Empty tree test completed successfully")
}

func TestRemoteGitPushNoChanges(t *testing.T) {
	ctx := context.Background()

	// Create a bare repository
	bareRepoPath := "testdata/no-changes-test-repo.git"
	exec.Command("rm", "-rf", bareRepoPath).Run()

	err := exec.Command("git", "init", "--bare", bareRepoPath).Run()
	if err != nil {
		t.Fatalf("Failed to create bare repo: %v", err)
	}

	// Connect with a user
	user := &GitUser{
		Name:  "Test Author",
		Email: "author@example.com",
	}

	rg, err := NewRemoteGitWithUser("file://"+bareRepoPath, user)
	if err != nil {
		t.Fatal(err)
	}

	// Initial commit with files
	initialObjects := toGitObjects(map[string]string{
		"file1.txt": "Content 1\n",
		"file2.txt": "Content 2\n",
	})

	err = rg.Push(ctx, "refs/heads/main", initialObjects, "Initial commit")
	if err != nil {
		t.Fatalf("Initial push failed: %v", err)
	}

	// Second commit: push with no changes (empty map)
	noChanges := map[string]GitObject{}

	err = rg.Push(ctx, "refs/heads/main", noChanges, "No changes")
	if err != nil {
		t.Fatalf("No changes push failed: %v", err)
	}

	// Verify files still exist
	refs, err := rg.BranchRefs(ctx)
	if err != nil {
		t.Fatal(err)
	}

	tree, err := rg.GetTree(ctx, refs[0].Hash)
	if err != nil {
		t.Fatal(err)
	}

	paths := tree.Paths()
	if len(paths) != 2 {
		t.Errorf("Expected 2 files, but got %d: %v", len(paths), paths)
	}

	t.Log("No changes test completed successfully")
}

func TestRemoteGitPushOnlyTombstones(t *testing.T) {
	ctx := context.Background()

	// Create a bare repository
	bareRepoPath := "testdata/only-tombstones-test-repo.git"
	exec.Command("rm", "-rf", bareRepoPath).Run()

	err := exec.Command("git", "init", "--bare", bareRepoPath).Run()
	if err != nil {
		t.Fatalf("Failed to create bare repo: %v", err)
	}

	// Connect with a user
	user := &GitUser{
		Name:  "Test Author",
		Email: "author@example.com",
	}

	rg, err := NewRemoteGitWithUser("file://"+bareRepoPath, user)
	if err != nil {
		t.Fatal(err)
	}

	// Initial commit with files
	initialObjects := toGitObjects(map[string]string{
		"file1.txt": "Content 1\n",
		"file2.txt": "Content 2\n",
	})

	err = rg.Push(ctx, "refs/heads/main", initialObjects, "Initial commit")
	if err != nil {
		t.Fatalf("Initial push failed: %v", err)
	}

	// Second commit: push with only tombstones for non-existent files
	onlyTombstones := map[string]GitObject{
		"nonexistent.txt": {
			Tombstone: true,
		},
	}

	err = rg.Push(ctx, "refs/heads/main", onlyTombstones, "Delete non-existent file")
	if err != nil {
		t.Fatalf("Tombstone-only push failed: %v", err)
	}

	// Verify original files still exist
	refs, err := rg.BranchRefs(ctx)
	if err != nil {
		t.Fatal(err)
	}

	tree, err := rg.GetTree(ctx, refs[0].Hash)
	if err != nil {
		t.Fatal(err)
	}

	paths := tree.Paths()
	if len(paths) != 2 {
		t.Errorf("Expected 2 files, but got %d: %v", len(paths), paths)
	}

	t.Log("Only tombstones test completed successfully")
}
