package refstore

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestGitBackend(t *testing.T) {
	doTestBackendSetGet(t, func() DocumentBackend {
		bareRepoPath, remoteURL := setupTestRepo(t, "set-get")
		be, err := NewGitBackend(context.Background(), bareRepoPath, remoteURL, "main", "", "", "")
		if err != nil {
			t.Fatal(err)
		}
		return be
	})
	doTestBackendMatch(t, func() DocumentBackend {
		bareRepoPath, remoteURL := setupTestRepo(t, "match")
		be, err := NewGitBackend(context.Background(), bareRepoPath, remoteURL, "main", "", "", "")
		if err != nil {
			t.Fatal(err)
		}
		return be
	})
	doTestBackendInfo(t, func() DocumentBackend {
		bareRepoPath, remoteURL := setupTestRepo(t, "info")
		be, err := NewGitBackend(context.Background(), bareRepoPath, remoteURL, "main", "", "", "")
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
	backend, err := NewGitBackend(context.Background(), bareRepoPath, remoteURL, "main", "", "", "")
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

// TestGitBackend_WorktreePollutesRemote verifies that ONLY the expected data files
// exist in the remote repository, with no worktree metadata pollution.
//
// This test writes a single file (test/data.json) to the backend, then clones the
// remote repo and verifies that ONLY this file exists, with no Git worktree metadata.
func TestGitBackend_WorktreePollutesRemote(t *testing.T) {
	
	bareRepoPath, remoteURL := setupTestRepo(t, "pollution-test")
	
	// Create a Git backend with a branch
	backend, err := NewGitBackend(context.Background(), bareRepoPath, remoteURL, "state", "", "", "")
	if err != nil {
		t.Fatalf("Failed to create backend: %v", err)
	}
	
	// Write some data to the backend
	bodyData := map[string]interface{}{
		"message": "test data",
	}
	bodyJSON, err := json.Marshal(bodyData)
	if err != nil {
		t.Fatalf("Failed to marshal body: %v", err)
	}
	
	testData := &StorageObject{
		Kind: "test",
		Body: bodyJSON,
	}
	
	err = backend.Set(context.Background(), nil, "Test commit", []SetRequest{
		{Path: "test/data.json", Doc: testData},
	})
	if err != nil {
		t.Fatalf("Failed to set data: %v", err)
	}
	
	// Clone the remote repo to verify what's actually in it
	clonePath := filepath.Join("testdata", "clones", "pollution-check")
	exec.Command("rm", "-rf", clonePath).Run()
	
	// Extract the file path from file:// URL
	remoteFilePath := remoteURL[7:] // Remove "file://"
	
	cmd := exec.Command("git", "clone", remoteFilePath, clonePath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to clone remote: %v\nOutput: %s", err, output)
	}
	
	// Switch to the state branch
	cmd = exec.Command("git", "-C", clonePath, "checkout", "state")
	output, err = cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to checkout state branch: %v\nOutput: %s", err, output)
	}
	
	// List all files in the clone
	cmd = exec.Command("git", "-C", clonePath, "ls-files")
	output, err = cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to list files: %v", err)
	}
	
	// Parse the list of files
	filesOutput := string(output)
	var actualFiles []string
	for _, line := range strings.Split(filesOutput, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			actualFiles = append(actualFiles, line)
		}
	}
	
	// Define expected files - should ONLY be our data file
	expectedFiles := []string{"test/data.json"}
	
	t.Logf("Files in remote repo: %v", actualFiles)
	t.Logf("Expected files: %v", expectedFiles)
	
	// Check that all expected files exist
	for _, expected := range expectedFiles {
		found := false
		for _, actual := range actualFiles {
			if actual == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected file '%s' not found in remote", expected)
		}
	}
	
	// Check that no unexpected files exist
	for _, actual := range actualFiles {
		expected := false
		for _, exp := range expectedFiles {
			if actual == exp {
				expected = true
				break
			}
		}
		if !expected {
			t.Errorf("POLLUTION DETECTED: Unexpected file '%s' found in remote repo", actual)
		}
	}
	
	// Verify file count matches
	if len(actualFiles) != len(expectedFiles) {
		t.Errorf("Expected %d files in remote, found %d", len(expectedFiles), len(actualFiles))
	}
}

func setupTestRepo(t *testing.T, name string) (bareRepoPath string, remoteURL string) {
	// Clean up all testdata for this test to handle stale data from previous runs
	// This includes old worktree locations that may have been created before the fix
	exec.Command("rm", "-rf", "testdata/local/"+name).Run()
	exec.Command("rm", "-rf", "testdata/local/worktrees/"+name).Run() // Old nested location
	exec.Command("rm", "-rf", "testdata/worktrees/"+name).Run()
	exec.Command("rm", "-rf", "testdata/remotes/"+name+".git").Run()
	exec.Command("rm", "-rf", "testdata/clones/"+name).Run()
	
	// Create a remote bare repository
	remotePath := filepath.Join("testdata", "remotes", name+".git")
	
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

	return localPath, "file://" + absRemotePath
}
