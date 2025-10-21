package git

import (
	"context"
	"os/exec"
	"testing"
)

func TestRemoteGitPushWithDeletion(t *testing.T) {
	ctx := context.Background()

	// Create a bare repository
	bareRepoPath := "testdata/deletion-test-repo.git"
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

	// Initial commit with multiple files
	initialObjects := map[string]GitObject{
		"README.md": {
			Content: []byte("# Test Repository\n"),
		},
		"src/main.go": {
			Content: []byte("package main\n\nfunc main() {}\n"),
		},
		"src/utils.go": {
			Content: []byte("package main\n\nfunc helper() {}\n"),
		},
		"docs/guide.md": {
			Content: []byte("# Guide\n"),
		},
	}

	err = rg.Push(ctx, "refs/heads/main", initialObjects, "Initial commit")
	if err != nil {
		t.Fatalf("Initial push failed: %v", err)
	}

	// Verify all files exist
	refs, err := rg.BranchRefs(ctx)
	if err != nil {
		t.Fatal(err)
	}

	tree, err := rg.GetTree(ctx, refs[0].Hash)
	if err != nil {
		t.Fatal(err)
	}

	for path := range initialObjects {
		node, err := tree.NodeAtPath(path)
		if err != nil {
			t.Errorf("File %s not found in tree: %v", path, err)
		}
		if node == nil {
			t.Errorf("File %s is nil", path)
		}
	}

	// Second commit: delete one file and modify another
	updateObjects := map[string]GitObject{
		"src/utils.go": {
			Tombstone: true, // Delete this file
		},
		"README.md": {
			Content: []byte("# Test Repository\n\nUpdated content\n"),
		},
	}

	err = rg.Push(ctx, "refs/heads/main", updateObjects, "Delete utils.go and update README")
	if err != nil {
		t.Fatalf("Second push failed: %v", err)
	}

	// Verify the deletion
	refs, err = rg.BranchRefs(ctx)
	if err != nil {
		t.Fatal(err)
	}

	tree, err = rg.GetTree(ctx, refs[0].Hash)
	if err != nil {
		t.Fatal(err)
	}

	// Check that utils.go is deleted
	node, err := tree.NodeAtPath("src/utils.go")
	if err != nil {
		t.Errorf("Error checking deleted file: %v", err)
	}
	if node != nil {
		t.Error("Expected src/utils.go to be deleted, but it still exists")
	}

	// Check that other files still exist
	node, err = tree.NodeAtPath("src/main.go")
	if err != nil {
		t.Errorf("Error checking src/main.go: %v", err)
	}
	if node == nil {
		t.Error("Expected src/main.go to still exist")
	}

	node, err = tree.NodeAtPath("docs/guide.md")
	if err != nil {
		t.Errorf("Error checking docs/guide.md: %v", err)
	}
	if node == nil {
		t.Error("Expected docs/guide.md to still exist")
	}

	// Check that README was updated
	node, err = tree.NodeAtPath("README.md")
	if err != nil {
		t.Errorf("Error checking README.md: %v", err)
	}
	if node == nil {
		t.Error("Expected README.md to exist")
	} else {
		content, err := rg.GetObject(ctx, node.Hash)
		if err != nil {
			t.Errorf("Failed to get README content: %v", err)
		}
		expected := "# Test Repository\n\nUpdated content\n"
		if string(content) != expected {
			t.Errorf("README content mismatch.\nExpected: %q\nGot: %q", expected, string(content))
		}
	}

	t.Log("File deletion test completed successfully")
}

func TestRemoteGitPushDeleteMultipleFiles(t *testing.T) {
	ctx := context.Background()

	// Create a bare repository
	bareRepoPath := "testdata/multi-deletion-test-repo.git"
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

	// Initial commit with many files
	initialObjects := toGitObjects(map[string]string{
		"file1.txt":     "Content 1\n",
		"file2.txt":     "Content 2\n",
		"file3.txt":     "Content 3\n",
		"dir/file4.txt": "Content 4\n",
		"dir/file5.txt": "Content 5\n",
	})

	err = rg.Push(ctx, "refs/heads/main", initialObjects, "Initial commit")
	if err != nil {
		t.Fatalf("Initial push failed: %v", err)
	}

	// Delete multiple files at once
	deleteObjects := map[string]GitObject{
		"file2.txt": {
			Tombstone: true,
		},
		"dir/file4.txt": {
			Tombstone: true,
		},
		"file3.txt": {
			Tombstone: true,
		},
	}

	err = rg.Push(ctx, "refs/heads/main", deleteObjects, "Delete multiple files")
	if err != nil {
		t.Fatalf("Delete push failed: %v", err)
	}

	// Verify deletions
	refs, err := rg.BranchRefs(ctx)
	if err != nil {
		t.Fatal(err)
	}

	tree, err := rg.GetTree(ctx, refs[0].Hash)
	if err != nil {
		t.Fatal(err)
	}

	// Check deleted files
	deletedFiles := []string{"file2.txt", "dir/file4.txt", "file3.txt"}
	for _, path := range deletedFiles {
		node, err := tree.NodeAtPath(path)
		if err != nil {
			t.Errorf("Error checking %s: %v", path, err)
		}
		if node != nil {
			t.Errorf("Expected %s to be deleted, but it still exists", path)
		}
	}

	// Check remaining files
	remainingFiles := []string{"file1.txt", "dir/file5.txt"}
	for _, path := range remainingFiles {
		node, err := tree.NodeAtPath(path)
		if err != nil {
			t.Errorf("Error checking %s: %v", path, err)
		}
		if node == nil {
			t.Errorf("Expected %s to still exist", path)
		}
	}

	t.Log("Multiple file deletion test completed successfully")
}

func TestRemoteGitPushDeleteAndRecreate(t *testing.T) {
	ctx := context.Background()

	// Create a bare repository
	bareRepoPath := "testdata/delete-recreate-test-repo.git"
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

	// Initial commit
	initialObjects := toGitObjects(map[string]string{
		"test.txt": "Version 1\n",
	})

	err = rg.Push(ctx, "refs/heads/main", initialObjects, "Initial commit")
	if err != nil {
		t.Fatalf("Initial push failed: %v", err)
	}

	// Delete the file
	deleteObjects := map[string]GitObject{
		"test.txt": {
			Tombstone: true,
		},
	}

	err = rg.Push(ctx, "refs/heads/main", deleteObjects, "Delete test.txt")
	if err != nil {
		t.Fatalf("Delete push failed: %v", err)
	}

	// Verify deletion
	refs, err := rg.BranchRefs(ctx)
	if err != nil {
		t.Fatal(err)
	}

	tree, err := rg.GetTree(ctx, refs[0].Hash)
	if err != nil {
		t.Fatal(err)
	}

	node, err := tree.NodeAtPath("test.txt")
	if err != nil {
		t.Errorf("Error checking test.txt: %v", err)
	}
	if node != nil {
		t.Error("Expected test.txt to be deleted")
	}

	// Recreate the file with new content
	recreateObjects := map[string]GitObject{
		"test.txt": {
			Content: []byte("Version 2\n"),
		},
	}

	err = rg.Push(ctx, "refs/heads/main", recreateObjects, "Recreate test.txt")
	if err != nil {
		t.Fatalf("Recreate push failed: %v", err)
	}

	// Verify recreation
	refs, err = rg.BranchRefs(ctx)
	if err != nil {
		t.Fatal(err)
	}

	tree, err = rg.GetTree(ctx, refs[0].Hash)
	if err != nil {
		t.Fatal(err)
	}

	node, err = tree.NodeAtPath("test.txt")
	if err != nil {
		t.Errorf("Error checking test.txt: %v", err)
	}
	if node == nil {
		t.Error("Expected test.txt to be recreated")
	} else {
		content, err := rg.GetObject(ctx, node.Hash)
		if err != nil {
			t.Errorf("Failed to get test.txt content: %v", err)
		}
		expected := "Version 2\n"
		if string(content) != expected {
			t.Errorf("Content mismatch.\nExpected: %q\nGot: %q", expected, string(content))
		}
	}

	t.Log("Delete and recreate test completed successfully")
}
