package refstore

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/charmbracelet/log"
)

func TestGoGitBackend(t *testing.T) {
	doTestBackendSetGet(t, func() DocumentBackend {
		bareRepoPath, remoteURL := setupTestRepo(t, "set-get", false)
		be, err := NewGoGitBackend(context.Background(), bareRepoPath, remoteURL, "main", "", "")
		if err != nil {
			t.Fatal(err)
		}
		return be
	})
	doTestBackendMatch(t, func() DocumentBackend {
		bareRepoPath, remoteURL := setupTestRepo(t, "match", false)
		be, err := NewGoGitBackend(context.Background(), bareRepoPath, remoteURL, "main", "", "")
		if err != nil {
			t.Fatal(err)
		}
		return be
	})
	doTestBackendInfo(t, func() DocumentBackend {
		bareRepoPath, remoteURL := setupTestRepo(t, "info", false)
		be, err := NewGoGitBackend(context.Background(), bareRepoPath, remoteURL, "main", "", "")
		if err != nil {
			t.Fatal(err)
		}
		return be
	})
	multiClientRepoPath, multiClientRemoteURL := setupTestRepo(t, "multiple-clients", false)
	doTestMultipleClients(t, func() DocumentBackend {
		be, err := NewGoGitBackend(context.Background(), multiClientRepoPath, multiClientRemoteURL, "main", "", "")
		if err != nil {
			t.Fatal(err)
		}
		return be
	})
}

func TestGoGitBackendCreatesBranch(t *testing.T) {
	log.SetOutput(logWriter(t))

	bareRepoPath, remoteURL := setupTestRepoWithBranch(t, "set-get-intent-branch", true, "main")
	doTestBackendSetGet(t, func() DocumentBackend {
		be, err := NewGoGitBackend(context.Background(), bareRepoPath, remoteURL, "intent", "", "")
		if err != nil {
			t.Fatalf("first init: %v", err)
		}
		return be
	})

	// Repeat to ensure the repo is reusable
	doTestBackendSetGet(t, func() DocumentBackend {
		be, err := NewGoGitBackend(context.Background(), bareRepoPath, remoteURL, "intent", "", "")
		if err != nil {
			t.Fatalf("second init: %v", err)
		}
		return be
	})

	// Check that the .ocuroot-store file is not present in the root of the branch
	cmd := exec.Command("git", "clone", "--branch", "intent", "--single-branch", remoteURL, "testdata/branch-test")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Logf("Output:\n%v", string(out))
		t.Fatalf("Failed to clone repo: %v", err)
	}
	defer os.RemoveAll("testdata/branch-test")

	_, err := os.Stat("testdata/branch-test/.ocuroot-store")
	if !os.IsNotExist(err) {
		t.Fatalf(".ocuroot-store file should not be present in the root of the branch")
	}
}

func TestGoGitBackendHandlesEmptyRepo(t *testing.T) {
	log.SetOutput(logWriter(t))

	bareRepoPath, remoteURL := setupTestRepo(t, "set-get-intent-branch", false)
	doTestBackendSetGet(t, func() DocumentBackend {
		be, err := NewGoGitBackend(context.Background(), bareRepoPath, remoteURL, "intent", "", "")
		if err != nil {
			t.Fatalf("first init: %v", err)
		}
		return be
	})

	// Repeat to ensure the repo is reusable
	doTestBackendSetGet(t, func() DocumentBackend {
		be, err := NewGoGitBackend(context.Background(), bareRepoPath, remoteURL, "intent", "", "")
		if err != nil {
			t.Fatalf("second init: %v", err)
		}
		return be
	})

	// Check that the .ocuroot-store file is not present in the root of the branch
	cmd := exec.Command("git", "clone", "--branch", "intent", "--single-branch", remoteURL, "testdata/branch-test")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Logf("Output:\n%v", string(out))
		t.Fatalf("Failed to clone repo: %v", err)
	}
	defer os.RemoveAll("testdata/branch-test")

	_, err := os.Stat("testdata/branch-test/.ocuroot-store")
	if !os.IsNotExist(err) {
		t.Fatalf(".ocuroot-store file should not be present in the root of the branch")
	}
}

func setupTestRepo(t *testing.T, name string, createBranch bool) (bareRepo string, remoteURL string) {
	return setupTestRepoWithBranch(t, name, createBranch, "main")
}

func setupTestRepoWithBranch(t *testing.T, name string, createBranch bool, branch string) (bareRepoPath string, remoteURL string) {
	// Clean up all testdata for this test to handle stale data from previous runs
	// This includes old worktree locations that may have been created before the fix
	exec.Command("rm", "-rf", "testdata/local/"+name).Run()
	exec.Command("rm", "-rf", "testdata/local/worktrees/"+name).Run() // Old nested location
	exec.Command("rm", "-rf", "testdata/worktrees/"+name).Run()
	exec.Command("rm", "-rf", "testdata/remotes/"+name+".git").Run()
	exec.Command("rm", "-rf", "testdata/clones/"+name).Run()
	exec.Command("rm", "-rf", "testdata/"+name).Run()

	// Create a remote bare repository
	remotePath := filepath.Join("testdata", "remotes", name+".git")

	err := exec.Command("git", "init", "--bare", remotePath, "-b", branch).Run()
	if err != nil {
		t.Fatalf("Failed to create remote bare repo: %v", err)
	}

	// If createBranch is true, initialize the bare repo with a branch
	if createBranch {
		// Create a temporary clone to initialize the branch
		tempClone := filepath.Join("testdata", "temp-"+name)
		defer exec.Command("rm", "-rf", tempClone).Run()

		// Clone the bare repo
		if err := exec.Command("git", "clone", remotePath, tempClone).Run(); err != nil {
			t.Fatalf("Failed to clone bare repo: %v", err)
		}

		// Create an initial commit in the temp clone
		cmd := exec.Command("git", "commit", "--allow-empty", "-m", "Initial commit")
		cmd.Dir = tempClone
		if err := cmd.Run(); err != nil {
			t.Fatalf("Failed to create initial commit: %v", err)
		}

		// Push to the bare repo
		cmd = exec.Command("git", "push", "origin", branch)
		cmd.Dir = tempClone
		if err := cmd.Run(); err != nil {
			t.Fatalf("Failed to push initial commit: %v", err)
		}
	}

	// Get absolute path for file:// URL
	absRemotePath, err := filepath.Abs(remotePath)
	if err != nil {
		t.Fatalf("Failed to get absolute path: %v", err)
	}

	// Create local bare repo path for the backend
	localPath := filepath.Join("testdata", "local", name)

	return localPath, "file://" + absRemotePath
}
