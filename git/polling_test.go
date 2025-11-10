package git

import (
	"context"
	"testing"
	"time"
)

// mockRemoteGit is a mock implementation of RemoteGit for testing
type mockRemoteGit struct {
	endpoint string
	refs     []Ref
}

func (m *mockRemoteGit) Endpoint() string {
	return m.endpoint
}

func (m *mockRemoteGit) BranchRefs(ctx context.Context) ([]Ref, error) {
	return m.refs, nil
}

func (m *mockRemoteGit) TagRefs(ctx context.Context) ([]Ref, error) {
	return nil, nil
}

func (m *mockRemoteGit) GetTree(ctx context.Context, ref string) (*TreeNode, error) {
	return nil, nil
}

func (m *mockRemoteGit) GetObject(ctx context.Context, hash string) ([]byte, error) {
	return nil, nil
}

func (m *mockRemoteGit) GetCommitMessage(ctx context.Context, hash string) (string, error) {
	return "", nil
}

func (m *mockRemoteGit) InvalidateConnection() {}

func TestPollRemote_EmptyHash(t *testing.T) {
	// Test that PollRemote doesn't call the callback when the branch doesn't exist (empty hash)
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	
	// Create a mock remote with no refs (simulating a branch that doesn't exist)
	remote := &mockRemoteGit{
		endpoint: "test-endpoint",
		refs:     []Ref{}, // Empty refs means the branch doesn't exist
	}
	
	callbackCalled := false
	callback := func(hash string) {
		callbackCalled = true
		if hash == "" {
			t.Error("Callback should not be called with empty hash")
		}
	}
	
	ticker := make(chan time.Time)
	close(ticker) // Close immediately so PollRemote exits
	
	err := PollRemote(ctx, remote, "main", callback, ticker)
	if err != nil && err != context.DeadlineExceeded {
		t.Fatalf("Unexpected error: %v", err)
	}
	
	if callbackCalled {
		t.Error("Callback should not have been called when branch doesn't exist")
	}
}

func TestPollRemote_ValidHash(t *testing.T) {
	// Test that PollRemote calls the callback when a valid hash exists
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	
	expectedHash := "abc123def456"
	remote := &mockRemoteGit{
		endpoint: "test-endpoint",
		refs: []Ref{
			{Name: "refs/heads/main", Hash: expectedHash},
		},
	}
	
	var receivedHash string
	callback := func(hash string) {
		receivedHash = hash
	}
	
	ticker := make(chan time.Time)
	close(ticker) // Close immediately so PollRemote exits after initial callback
	
	err := PollRemote(ctx, remote, "main", callback, ticker)
	if err != nil && err != context.DeadlineExceeded {
		t.Fatalf("Unexpected error: %v", err)
	}
	
	if receivedHash != expectedHash {
		t.Errorf("Expected hash %q, got %q", expectedHash, receivedHash)
	}
}
