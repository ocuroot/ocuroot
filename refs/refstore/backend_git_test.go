package refstore

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestGitBackend(t *testing.T) {
	doTestBackendSetGet(t, func() DocumentBackend {
		bareRepoPath, remoteURL := setupTestRepo(t, "set-get")
		be, err := NewGitBackend(context.Background(), bareRepoPath, remoteURL, "main", "", "")
		if err != nil {
			t.Fatal(err)
		}
		return be
	})
	doTestBackendMatch(t, func() DocumentBackend {
		bareRepoPath, remoteURL := setupTestRepo(t, "match")
		be, err := NewGitBackend(context.Background(), bareRepoPath, remoteURL, "main", "", "")
		if err != nil {
			t.Fatal(err)
		}
		return be
	})
	doTestBackendInfo(t, func() DocumentBackend {
		bareRepoPath, remoteURL := setupTestRepo(t, "info")
		be, err := NewGitBackend(context.Background(), bareRepoPath, remoteURL, "main", "", "")
		if err != nil {
			t.Fatal(err)
		}
		return be
	})
}

// TestGitBackendInfoFileFormat verifies that the info file is written in the correct format
// (as StoreInfo directly, not wrapped in a StorageObject)
func TestGitBackendInfoFileFormat(t *testing.T) {
	bareRepoPath, remoteURL := setupTestRepo(t, "info-format")
	backend, err := NewGitBackend(context.Background(), bareRepoPath, remoteURL, "main", "", "")
	if err != nil {
		t.Fatal(err)
	}

	// Set info with tags
	testInfo := &StoreInfo{
		Version: 3,
		Tags: map[string]struct{}{
			"test":   {},
			"intent": {},
		},
	}
	
	infoBytes, err := json.Marshal(testInfo)
	if err != nil {
		t.Fatal(err)
	}
	
	if err := backend.SetBytes(context.Background(), storeInfoFile, infoBytes); err != nil {
		t.Fatal(err)
	}

	// Read the raw file from the worktree to verify format
	gitBackend := backend.(*gitBackend)
	infoFilePath := filepath.Join(gitBackend.worktreePath, storeInfoFile)
	
	rawContent, err := os.ReadFile(infoFilePath)
	if err != nil {
		t.Fatalf("Failed to read info file: %v", err)
	}

	// Verify it's valid StoreInfo JSON (not wrapped in StorageObject)
	var info StoreInfo
	if err := json.Unmarshal(rawContent, &info); err != nil {
		t.Fatalf("Info file is not valid StoreInfo JSON: %v\nContent: %s", err, string(rawContent))
	}

	// Verify the content matches what we set
	if info.Version != 3 {
		t.Errorf("expected version 3, got %d", info.Version)
	}

	if len(info.Tags) != 2 {
		t.Errorf("expected 2 tags, got %d", len(info.Tags))
	}

	if _, ok := info.Tags["test"]; !ok {
		t.Errorf("expected 'test' tag to be present")
	}

	if _, ok := info.Tags["intent"]; !ok {
		t.Errorf("expected 'intent' tag to be present")
	}

	// Verify it's NOT a StorageObject by checking for the "kind" field
	var rawMap map[string]interface{}
	if err := json.Unmarshal(rawContent, &rawMap); err != nil {
		t.Fatalf("Failed to unmarshal as map: %v", err)
	}

	if _, hasKind := rawMap["kind"]; hasKind {
		t.Errorf("Info file should not have 'kind' field (it's wrapped in StorageObject). Content: %s", string(rawContent))
	}

	if _, hasBody := rawMap["body"]; hasBody {
		t.Errorf("Info file should not have 'body' field (it's wrapped in StorageObject). Content: %s", string(rawContent))
	}
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
