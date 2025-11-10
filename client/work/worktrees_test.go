package work

import (
	"context"
	"testing"
)

func TestCloneRepo_EmptyCommit(t *testing.T) {
	// Test that CloneRepo returns an error when commit is empty
	_, err := CloneRepo(context.Background(), []string{"https://example.com/repo.git"}, "")
	if err == nil {
		t.Fatal("Expected error when commit is empty, got nil")
	}
	
	expectedMsg := "commit hash cannot be empty"
	if err.Error() != expectedMsg {
		t.Errorf("Expected error message %q, got %q", expectedMsg, err.Error())
	}
}
