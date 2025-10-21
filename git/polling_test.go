package git

import (
	"context"
	"os/exec"
	"testing"
	"time"
)

func TestPollRemote(t *testing.T) {
	ctx := context.Background()

	// Create a bare repository
	bareRepoPath := "testdata/poll-test-repo.git"
	exec.Command("rm", "-rf", bareRepoPath).Run()

	err := exec.Command("git", "init", "--bare", bareRepoPath).Run()
	if err != nil {
		t.Fatalf("Failed to create bare repo: %v", err)
	}

	// Create RemoteGit connection
	// Note: Using a single instance for both push and poll. In production,
	// these would typically be separate processes/connections, but the file://
	// protocol has locking issues with concurrent access from multiple instances.
	user := &GitUser{
		Name:  "Test User",
		Email: "test@example.com",
	}

	rg, err := NewRemoteGitWithUser("file://"+bareRepoPath, user)
	if err != nil {
		t.Fatal(err)
	}

	// Push initial commit
	initialObjects := map[string]string{
		"file1.txt": "Initial content\n",
	}
	err = rg.Push(ctx, "refs/heads/main", toGitObjects(initialObjects), "Initial commit")
	if err != nil {
		t.Fatalf("Initial push failed: %v", err)
	}

	// Get initial hash
	refs, err := rg.BranchRefs(ctx)
	if err != nil {
		t.Fatal(err)
	}
	initialHash := refs[0].Hash

	// Track callbacks via channel
	callbackChan := make(chan string, 10)
	callback := func(hash string) {
		t.Logf("Callback received hash: %s", hash)
		callbackChan <- hash
	}

	// Create ticker channel
	ticker := make(chan time.Time)

	// Start polling in background
	pollCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	pollErr := make(chan error, 1)
	go func() {
		pollErr <- PollRemote(pollCtx, rg, "refs/heads/main", callback, ticker)
	}()

	// Push second commit
	secondObjects := map[string]string{
		"file1.txt": "Updated content\n",
		"file2.txt": "New file\n",
	}
	err = rg.Push(ctx, "refs/heads/main", toGitObjects(secondObjects), "Second commit")
	if err != nil {
		t.Fatalf("Second push failed: %v", err)
	}

	// Get second hash
	refs, err = rg.BranchRefs(ctx)
	if err != nil {
		t.Fatal(err)
	}
	secondHash := refs[0].Hash

	// Collect callbacks as they come in
	var receivedHashes []string
	receivedHashes = append(receivedHashes, <-callbackChan) // Initial callback

	// Send multiple ticker signals - should only get one callback per actual change
	// The ticker blocks, so each send ensures at least one poll is processed
	ticker <- time.Now()
	receivedHashes = append(receivedHashes, <-callbackChan) // Second commit callback

	// Send more ticker signals with no changes - should not get callbacks
	ticker <- time.Now()
	ticker <- time.Now()

	// Push third commit
	thirdObjects := map[string]string{
		"file1.txt": "Final content\n",
		"file2.txt": "Updated file\n",
		"file3.txt": "Another new file\n",
	}
	err = rg.Push(ctx, "refs/heads/main", toGitObjects(thirdObjects), "Third commit")
	if err != nil {
		t.Fatalf("Third push failed: %v", err)
	}

	// Get third hash
	refs, err = rg.BranchRefs(ctx)
	if err != nil {
		t.Fatal(err)
	}
	thirdHash := refs[0].Hash

	// Poll for third commit
	ticker <- time.Now()
	receivedHashes = append(receivedHashes, <-callbackChan) // Third commit callback

	// Send more ticker signals with no changes - should not get callbacks
	ticker <- time.Now()

	// Close ticker to signal completion
	close(ticker)

	// Wait for poller to exit
	err = <-pollErr
	if err != nil {
		t.Errorf("Expected nil error after closing ticker, got: %v", err)
	}

	// Verify no additional callbacks were buffered
	select {
	case hash := <-callbackChan:
		t.Errorf("Got unexpected additional callback: %s", hash)
	default:
		// Good - no extra callbacks
	}

	if len(receivedHashes) != 3 {
		t.Fatalf("Expected 3 callbacks, got %d: %v", len(receivedHashes), receivedHashes)
	}

	if receivedHashes[0] != initialHash {
		t.Errorf("First callback hash mismatch: expected %s, got %s", initialHash, receivedHashes[0])
	}
	if receivedHashes[1] != secondHash {
		t.Errorf("Second callback hash mismatch: expected %s, got %s", secondHash, receivedHashes[1])
	}
	if receivedHashes[2] != thirdHash {
		t.Errorf("Third callback hash mismatch: expected %s, got %s", thirdHash, receivedHashes[2])
	}

	t.Log("Poll test completed successfully")
}

func TestPollRemoteNoChanges(t *testing.T) {
	ctx := context.Background()

	// Create a bare repository
	bareRepoPath := "testdata/poll-no-changes-repo.git"
	exec.Command("rm", "-rf", bareRepoPath).Run()

	err := exec.Command("git", "init", "--bare", bareRepoPath).Run()
	if err != nil {
		t.Fatalf("Failed to create bare repo: %v", err)
	}

	// Create RemoteGit connection
	user := &GitUser{
		Name:  "Test User",
		Email: "test@example.com",
	}

	rg, err := NewRemoteGitWithUser("file://"+bareRepoPath, user)
	if err != nil {
		t.Fatal(err)
	}

	// Push initial commit
	initialObjects := map[string]string{
		"file1.txt": "Content\n",
	}
	err = rg.Push(ctx, "refs/heads/main", toGitObjects(initialObjects), "Initial commit")
	if err != nil {
		t.Fatalf("Initial push failed: %v", err)
	}

	// Track callbacks via channel
	callbackChan := make(chan string, 10)
	callback := func(hash string) {
		t.Logf("Callback: %s", hash)
		callbackChan <- hash
	}

	// Create ticker channel
	ticker := make(chan time.Time)

	// Start polling in background
	pollCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	go func() {
		PollRemote(pollCtx, rg, "refs/heads/main", callback, ticker)
	}()

	// Wait for initial callback
	<-callbackChan

	// Trigger multiple polls with no changes
	for i := 0; i < 5; i++ {
		ticker <- time.Now()
		select {
		case hash := <-callbackChan:
			t.Errorf("Expected no callback on poll %d with no changes, but got: %s", i+1, hash)
		case <-time.After(50 * time.Millisecond):
			// Good - no callback received
		}
	}

	cancel()
	t.Log("No-changes poll test completed successfully")
}

func TestPollRemoteContextCancellation(t *testing.T) {
	ctx := context.Background()

	// Create a bare repository
	bareRepoPath := "testdata/poll-cancel-repo.git"
	exec.Command("rm", "-rf", bareRepoPath).Run()

	err := exec.Command("git", "init", "--bare", bareRepoPath).Run()
	if err != nil {
		t.Fatalf("Failed to create bare repo: %v", err)
	}

	// Create RemoteGit connection
	user := &GitUser{
		Name:  "Test User",
		Email: "test@example.com",
	}

	rg, err := NewRemoteGitWithUser("file://"+bareRepoPath, user)
	if err != nil {
		t.Fatal(err)
	}

	// Push initial commit
	initialObjects := map[string]string{
		"file1.txt": "Content\n",
	}
	err = rg.Push(ctx, "refs/heads/main", toGitObjects(initialObjects), "Initial commit")
	if err != nil {
		t.Fatalf("Initial push failed: %v", err)
	}

	// Track callbacks via channel
	callbackChan := make(chan string, 1)
	callback := func(hash string) {
		callbackChan <- hash
	}

	// Create ticker channel
	ticker := make(chan time.Time)

	// Start polling with cancellable context
	pollCtx, cancel := context.WithCancel(ctx)

	pollErr := make(chan error, 1)
	go func() {
		pollErr <- PollRemote(pollCtx, rg, "refs/heads/main", callback, ticker)
	}()

	// Wait for initial callback
	<-callbackChan

	// Cancel immediately
	cancel()

	// Verify it returns with context.Canceled
	select {
	case err := <-pollErr:
		if err != context.Canceled {
			t.Errorf("Expected context.Canceled, got: %v", err)
		}
	case <-time.After(200 * time.Millisecond):
		t.Error("PollRemote did not return after context cancellation")
	}

	t.Log("Context cancellation test completed successfully")
}

func TestPollRemoteWithSeparateClients(t *testing.T) {
	ctx := context.Background()

	// Create a bare repository
	bareRepoPath := "testdata/poll-invalidate-repo.git"
	exec.Command("rm", "-rf", bareRepoPath).Run()

	err := exec.Command("git", "init", "--bare", bareRepoPath).Run()
	if err != nil {
		t.Fatalf("Failed to create bare repo: %v", err)
	}

	// Create RemoteGit connection for pushing
	user := &GitUser{
		Name:  "Test User",
		Email: "test@example.com",
	}

	rgPush, err := NewRemoteGitWithUser("file://"+bareRepoPath, user)
	if err != nil {
		t.Fatal(err)
	}

	// Create separate RemoteGit connection for polling
	rgPoll, err := NewRemoteGitWithUser("file://"+bareRepoPath, user)
	if err != nil {
		t.Fatal(err)
	}

	// Push initial commit
	initialObjects := map[string]string{
		"file1.txt": "Initial content\n",
	}
	err = rgPush.Push(ctx, "refs/heads/main", toGitObjects(initialObjects), "Initial commit")
	if err != nil {
		t.Fatalf("Initial push failed: %v", err)
	}

	// Get initial hash
	refs, err := rgPush.BranchRefs(ctx)
	if err != nil {
		t.Fatal(err)
	}
	initialHash := refs[0].Hash

	// Track callbacks via channel
	callbackChan := make(chan string, 10)
	callback := func(hash string) {
		t.Logf("Callback received hash: %s", hash)
		callbackChan <- hash
	}

	// Create ticker channel
	ticker := make(chan time.Time)

	// Start polling in background (invalidation enabled via global variable)
	pollCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	pollErr := make(chan error, 1)
	go func() {
		pollErr <- PollRemote(pollCtx, rgPoll, "refs/heads/main", callback, ticker)
	}()

	// Push second commit
	secondObjects := map[string]string{
		"file1.txt": "Updated content\n",
		"file2.txt": "New file\n",
	}
	err = rgPush.Push(ctx, "refs/heads/main", toGitObjects(secondObjects), "Second commit")
	if err != nil {
		t.Fatalf("Second push failed: %v", err)
	}

	// Get second hash
	refs, err = rgPush.BranchRefs(ctx)
	if err != nil {
		t.Fatal(err)
	}
	secondHash := refs[0].Hash

	// Collect callbacks as they come in
	var receivedHashes []string
	receivedHashes = append(receivedHashes, <-callbackChan) // Initial callback

	// Send ticker signal - should get second commit callback
	ticker <- time.Now()
	receivedHashes = append(receivedHashes, <-callbackChan) // Second commit callback

	// Push third commit
	thirdObjects := map[string]string{
		"file1.txt": "Final content\n",
		"file2.txt": "Updated file\n",
		"file3.txt": "Another new file\n",
	}
	err = rgPush.Push(ctx, "refs/heads/main", toGitObjects(thirdObjects), "Third commit")
	if err != nil {
		t.Fatalf("Third push failed: %v", err)
	}

	// Get third hash
	refs, err = rgPush.BranchRefs(ctx)
	if err != nil {
		t.Fatal(err)
	}
	thirdHash := refs[0].Hash

	// Poll for third commit
	ticker <- time.Now()
	receivedHashes = append(receivedHashes, <-callbackChan) // Third commit callback

	// Close ticker to signal completion
	close(ticker)

	// Wait for poller to exit
	err = <-pollErr
	if err != nil {
		t.Errorf("Expected nil error after closing ticker, got: %v", err)
	}

	// Verify we got exactly 3 callbacks
	if len(receivedHashes) != 3 {
		t.Fatalf("Expected 3 callbacks, got %d: %v", len(receivedHashes), receivedHashes)
	}

	if receivedHashes[0] != initialHash {
		t.Errorf("First callback hash mismatch: expected %s, got %s", initialHash, receivedHashes[0])
	}
	if receivedHashes[1] != secondHash {
		t.Errorf("Second callback hash mismatch: expected %s, got %s", secondHash, receivedHashes[1])
	}
	if receivedHashes[2] != thirdHash {
		t.Errorf("Third callback hash mismatch: expected %s, got %s", thirdHash, receivedHashes[2])
	}

	t.Log("Poll with invalidation test completed successfully")
}
