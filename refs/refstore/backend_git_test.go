package refstore

import (
	"context"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestGitBackend(t *testing.T) {
	doTestBackendSetGet(t, func() DocumentBackend {
		bareRepoPath, remoteURL := setupTestRepo(t, "set-get")
		be, err := NewGitBackend(context.Background(), bareRepoPath, remoteURL, "main")
		if err != nil {
			t.Fatal(err)
		}
		return be
	})
	doTestBackendMatch(t, func() DocumentBackend {
		bareRepoPath, remoteURL := setupTestRepo(t, "match")
		be, err := NewGitBackend(context.Background(), bareRepoPath, remoteURL, "main")
		if err != nil {
			t.Fatal(err)
		}
		return be
	})
	doTestBackendInfo(t, func() DocumentBackend {
		bareRepoPath, remoteURL := setupTestRepo(t, "info")
		be, err := NewGitBackend(context.Background(), bareRepoPath, remoteURL, "main")
		if err != nil {
			t.Fatal(err)
		}
		return be
	})
}

func setupTestRepo(t *testing.T, name string) (bareRepoPath string, remoteURL string) {
	// Create a remote bare repository
	remotePath := filepath.Join("testdata", "remotes", name+".git")
	exec.Command("rm", "-rf", remotePath).Run()
	
	err := exec.Command("git", "init", "--bare", remotePath).Run()
	if err != nil {
		t.Fatalf("Failed to create remote bare repo: %v", err)
	}

	// Get absolute path for file:// URL
	absRemotePath, err := filepath.Abs(remotePath)
	if err != nil {
		t.Fatalf("Failed to get absolute path: %v", err)
	}

	// Create local bare repo path for the backend
	localPath := filepath.Join("testdata", "local", name)
	exec.Command("rm", "-rf", localPath).Run()

	return localPath, "file://" + absRemotePath
}
