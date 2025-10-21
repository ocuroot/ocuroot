package refstore

import (
	"context"
	"os/exec"
	"testing"

	"github.com/ocuroot/ocuroot/git"
)

func TestGitBackend(t *testing.T) {
	doTestBackendSetGet(t, func() DocumentBackend {
		be, err := NewGitBackend(context.Background(), remoteWithBareRepoAtDir(t, "git/set-get.git"), "main")
		if err != nil {
			t.Fatal(err)
		}
		return be
	})
	doTestBackendMatch(t, func() DocumentBackend {
		be, err := NewGitBackend(context.Background(), remoteWithBareRepoAtDir(t, "git/match.git"), "main")
		if err != nil {
			t.Fatal(err)
		}
		return be
	})
	doTestBackendInfo(t, func() DocumentBackend {
		be, err := NewGitBackend(context.Background(), remoteWithBareRepoAtDir(t, "git/info.git"), "main")
		if err != nil {
			t.Fatal(err)
		}
		return be
	})
}

func remoteWithBareRepoAtDir(t *testing.T, dir string) git.RemoteGit {
	// Create a bare repository under testdata for review
	bareRepoPath := "testdata/" + dir

	// Remove if it exists from previous run
	exec.Command("rm", "-rf", bareRepoPath).Run()

	// Initialize bare repository using git command
	err := exec.Command("git", "init", "--bare", bareRepoPath).Run()
	if err != nil {
		t.Fatalf("Failed to create bare repo: %v", err)
	}

	// Connect to the bare repository with a user
	user := &git.GitUser{
		Name:  "Test Author",
		Email: "author@example.com",
	}

	rg, err := git.NewRemoteGitWithUser("file://"+bareRepoPath, user)
	if err != nil {
		t.Fatal(err)
	}
	return rg
}
