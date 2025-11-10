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

	// Read the raw file from the clone to verify format
	gitBackend := backend.(*gitBackend)
	infoFilePath := filepath.Join(gitBackend.clonePath, storeInfoFile)
	
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
	defer backend.(*gitBackend).Close()
	
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
	
	// List files directly from the bare repo using git ls-tree
	remoteFilePath := remoteURL[7:] // Remove "file://"
	cmd := exec.Command("git", "-C", remoteFilePath, "ls-tree", "-r", "--name-only", "state")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to list files in state branch: %v\nOutput: %s", err, output)
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

// TestGitBackend_SerialAccess tests multiple backends accessing the same remote in series
func TestGitBackend_SerialAccess(t *testing.T) {
	bareRepoPath, remoteURL := setupTestRepo(t, "serial-test")
	
	// Create first backend and write data
	backend1, err := NewGitBackend(context.Background(), bareRepoPath, remoteURL, "main", "data", "", "")
	if err != nil {
		t.Fatalf("Failed to create backend1: %v", err)
	}
	defer backend1.(*gitBackend).Close()
	
	doc1 := &StorageObject{
		Kind: "test",
		Body: []byte(`{"value": "from backend1"}`),
	}
	
	err = backend1.Set(context.Background(), nil, "Backend1 commit", []SetRequest{
		{Path: "file1.json", Doc: doc1},
	})
	if err != nil {
		t.Fatalf("Backend1 Set failed: %v", err)
	}
	
	// Create second backend and verify it sees the first backend's changes
	backend2, err := NewGitBackend(context.Background(), bareRepoPath, remoteURL, "main", "data", "", "")
	if err != nil {
		t.Fatalf("Failed to create backend2: %v", err)
	}
	defer backend2.(*gitBackend).Close()
	
	data, err := backend2.GetBytes(context.Background(), "file1.json")
	if err != nil {
		t.Fatalf("Backend2 GetBytes failed: %v", err)
	}
	
	var retrieved StorageObject
	if err := json.Unmarshal(data, &retrieved); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}
	
	if string(retrieved.Body) != `{"value":"from backend1"}` {
		t.Errorf("Backend2 didn't see Backend1's changes. Got: %s", string(retrieved.Body))
	}
	
	// Backend2 makes its own change
	doc2 := &StorageObject{
		Kind: "test",
		Body: []byte(`{"value": "from backend2"}`),
	}
	
	err = backend2.Set(context.Background(), nil, "Backend2 commit", []SetRequest{
		{Path: "file2.json", Doc: doc2},
	})
	if err != nil {
		t.Fatalf("Backend2 Set failed: %v", err)
	}
	
	// Create third backend and verify it sees both changes
	backend3, err := NewGitBackend(context.Background(), bareRepoPath, remoteURL, "main", "data", "", "")
	if err != nil {
		t.Fatalf("Failed to create backend3: %v", err)
	}
	defer backend3.(*gitBackend).Close()
	
	data1, err := backend3.GetBytes(context.Background(), "file1.json")
	if err != nil {
		t.Fatalf("Backend3 GetBytes file1 failed: %v", err)
	}
	
	data2, err := backend3.GetBytes(context.Background(), "file2.json")
	if err != nil {
		t.Fatalf("Backend3 GetBytes file2 failed: %v", err)
	}
	
	if string(data1) != string(data) {
		t.Error("Backend3 didn't see Backend1's file")
	}
	
	var retrieved2 StorageObject
	if err := json.Unmarshal(data2, &retrieved2); err != nil {
		t.Fatalf("Failed to unmarshal file2: %v", err)
	}
	
	if string(retrieved2.Body) != `{"value":"from backend2"}` {
		t.Errorf("Backend3 didn't see Backend2's changes. Got: %s", string(retrieved2.Body))
	}
}

// TestGitBackend_UpstreamChanges tests that backends can handle external changes to the remote
func TestGitBackend_UpstreamChanges(t *testing.T) {
	bareRepoPath, remoteURL := setupTestRepo(t, "upstream-test")
	
	// Create a backend and write initial data
	backend1, err := NewGitBackend(context.Background(), bareRepoPath, remoteURL, "main", "data", "", "")
	if err != nil {
		t.Fatalf("Failed to create backend1: %v", err)
	}
	defer backend1.(*gitBackend).Close()
	
	doc1 := &StorageObject{
		Kind: "test",
		Body: []byte(`{"value": "initial"}`),
	}
	
	err = backend1.Set(context.Background(), nil, "Initial commit", []SetRequest{
		{Path: "file.json", Doc: doc1},
	})
	if err != nil {
		t.Fatalf("Backend1 Set failed: %v", err)
	}
	
	// Simulate an external change to the remote by cloning, modifying, and pushing
	remoteFilePath := remoteURL[7:] // Remove "file://"
	externalClone := filepath.Join("testdata", "clones", "external")
	exec.Command("rm", "-rf", externalClone).Run()
	
	cmd := exec.Command("git", "clone", remoteFilePath, externalClone)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("Failed to clone for external change: %v\nOutput: %s", err, output)
	}
	defer exec.Command("rm", "-rf", externalClone).Run()
	
	// Make an external change
	externalFile := filepath.Join(externalClone, "data", "external.json")
	externalDoc := &StorageObject{
		Kind: "test",
		Body: []byte(`{"value": "external change"}`),
	}
	externalJSON, _ := json.Marshal(externalDoc)
	
	if err := os.MkdirAll(filepath.Dir(externalFile), 0755); err != nil {
		t.Fatalf("Failed to create dir: %v", err)
	}
	if err := os.WriteFile(externalFile, externalJSON, 0644); err != nil {
		t.Fatalf("Failed to write external file: %v", err)
	}
	
	cmd = exec.Command("git", "-C", externalClone, "add", ".")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("External git add failed: %v\nOutput: %s", err, output)
	}
	
	cmd = exec.Command("git", "-C", externalClone, "commit", "-m", "External change")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("External git commit failed: %v\nOutput: %s", err, output)
	}
	
	cmd = exec.Command("git", "-C", externalClone, "push")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("External git push failed: %v\nOutput: %s", err, output)
	}
	
	// Now backend1 should be able to pull the external change and make another commit
	doc2 := &StorageObject{
		Kind: "test",
		Body: []byte(`{"value": "after external change"}`),
	}
	
	err = backend1.Set(context.Background(), nil, "After external change", []SetRequest{
		{Path: "file.json", Doc: doc2},
	})
	if err != nil {
		t.Fatalf("Backend1 Set after external change failed: %v", err)
	}
	
	// Verify backend1 can see the external file
	externalData, err := backend1.GetBytes(context.Background(), "external.json")
	if err != nil {
		t.Fatalf("Backend1 GetBytes external.json failed: %v", err)
	}
	
	var retrievedExternal StorageObject
	if err := json.Unmarshal(externalData, &retrievedExternal); err != nil {
		t.Fatalf("Failed to unmarshal external: %v", err)
	}
	
	if string(retrievedExternal.Body) != `{"value":"external change"}` {
		t.Errorf("Backend1 didn't see external change. Got: %s", string(retrievedExternal.Body))
	}
	
	// Create a new backend and verify it sees all changes
	backend2, err := NewGitBackend(context.Background(), bareRepoPath, remoteURL, "main", "data", "", "")
	if err != nil {
		t.Fatalf("Failed to create backend2: %v", err)
	}
	defer backend2.(*gitBackend).Close()
	
	fileData, err := backend2.GetBytes(context.Background(), "file.json")
	if err != nil {
		t.Fatalf("Backend2 GetBytes file.json failed: %v", err)
	}
	
	var retrievedFile StorageObject
	if err := json.Unmarshal(fileData, &retrievedFile); err != nil {
		t.Fatalf("Failed to unmarshal file: %v", err)
	}
	
	if string(retrievedFile.Body) != `{"value":"after external change"}` {
		t.Errorf("Backend2 didn't see latest change. Got: %s", string(retrievedFile.Body))
	}
	
	externalData2, err := backend2.GetBytes(context.Background(), "external.json")
	if err != nil {
		t.Fatalf("Backend2 GetBytes external.json failed: %v", err)
	}
	
	if string(externalData2) != string(externalData) {
		t.Error("Backend2 didn't see external file")
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
