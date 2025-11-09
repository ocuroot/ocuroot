package client

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFindRoot_WithBoundaryMarker(t *testing.T) {
	// Create a temporary directory structure:
	// tmpdir/
	//   parent/
	//     repo.ocu.star
	//     .ocuroot-dir-boundary  <- boundary blocks search from going to parent
	//     child/
	//       subdir/  <- search from here should NOT find parent repo
	
	tmpDir := t.TempDir()
	
	// Create parent repo.ocu.star (above the boundary)
	parentDir := filepath.Join(tmpDir, "parent")
	if err := os.MkdirAll(parentDir, 0755); err != nil {
		t.Fatalf("Failed to create parent dir: %v", err)
	}
	parentRepoFile := filepath.Join(parentDir, "repo.ocu.star")
	if err := os.WriteFile(parentRepoFile, []byte("# parent"), 0644); err != nil {
		t.Fatalf("Failed to create parent repo file: %v", err)
	}
	
	// Create boundary marker in parent (blocks upward search from child)
	boundaryPath := filepath.Join(parentDir, ".ocuroot-dir-boundary")
	if err := os.WriteFile(boundaryPath, []byte{}, 0644); err != nil {
		t.Fatalf("Failed to create boundary marker: %v", err)
	}
	
	// Create child directory without repo.ocu.star
	childDir := filepath.Join(parentDir, "child", "subdir")
	if err := os.MkdirAll(childDir, 0755); err != nil {
		t.Fatalf("Failed to create child dir: %v", err)
	}
	
	// Test: Search from child should NOT find parent repo (blocked by boundary)
	_, err := FindSourceRepoRoot(childDir)
	if err != ErrRootNotFound {
		t.Errorf("Expected ErrRootNotFound when crossing boundary, got: %v", err)
	}
}

func TestFindRoot_WithoutBoundaryMarker(t *testing.T) {
	// Create a temporary directory structure without boundary:
	// tmpdir/
	//   parent/
	//     repo.ocu.star
	//     child/
	//       subdir/
	
	tmpDir := t.TempDir()
	
	// Create parent repo.ocu.star
	parentDir := filepath.Join(tmpDir, "parent")
	if err := os.MkdirAll(parentDir, 0755); err != nil {
		t.Fatalf("Failed to create parent dir: %v", err)
	}
	parentRepoFile := filepath.Join(parentDir, "repo.ocu.star")
	if err := os.WriteFile(parentRepoFile, []byte("# parent"), 0644); err != nil {
		t.Fatalf("Failed to create parent repo file: %v", err)
	}
	
	// Create child subdirectory (no repo.ocu.star)
	childDir := filepath.Join(parentDir, "child", "subdir")
	if err := os.MkdirAll(childDir, 0755); err != nil {
		t.Fatalf("Failed to create child dir: %v", err)
	}
	
	// Search from child directory should find parent repo (no boundary blocking)
	foundRoot, err := FindSourceRepoRoot(childDir)
	if err != nil {
		t.Errorf("Expected to find parent repo, got error: %v", err)
	}
	if foundRoot != parentDir {
		t.Errorf("Expected root %s, got %s", parentDir, foundRoot)
	}
}

func TestFindRoot_BoundaryInMiddle(t *testing.T) {
	// Create a temporary directory structure:
	// tmpdir/
	//   grandparent/
	//     repo.ocu.star
	//     parent/
	//       .ocuroot-dir-boundary
	//       child/
	//         repo.ocu.star
	
	tmpDir := t.TempDir()
	
	// Create grandparent repo.ocu.star
	grandparentDir := filepath.Join(tmpDir, "grandparent")
	if err := os.MkdirAll(grandparentDir, 0755); err != nil {
		t.Fatalf("Failed to create grandparent dir: %v", err)
	}
	grandparentRepoFile := filepath.Join(grandparentDir, "repo.ocu.star")
	if err := os.WriteFile(grandparentRepoFile, []byte("# grandparent"), 0644); err != nil {
		t.Fatalf("Failed to create grandparent repo file: %v", err)
	}
	
	// Create boundary marker in parent
	parentDir := filepath.Join(grandparentDir, "parent")
	if err := os.MkdirAll(parentDir, 0755); err != nil {
		t.Fatalf("Failed to create parent dir: %v", err)
	}
	boundaryPath := filepath.Join(parentDir, ".ocuroot-dir-boundary")
	if err := os.WriteFile(boundaryPath, []byte{}, 0644); err != nil {
		t.Fatalf("Failed to create boundary marker: %v", err)
	}
	
	// Create child repo.ocu.star
	childDir := filepath.Join(parentDir, "child")
	if err := os.MkdirAll(childDir, 0755); err != nil {
		t.Fatalf("Failed to create child dir: %v", err)
	}
	childRepoFile := filepath.Join(childDir, "repo.ocu.star")
	if err := os.WriteFile(childRepoFile, []byte("# child"), 0644); err != nil {
		t.Fatalf("Failed to create child repo file: %v", err)
	}
	
	// Search from child should find child repo (not blocked by boundary)
	foundRoot, err := FindSourceRepoRoot(childDir)
	if err != nil {
		t.Errorf("Expected to find child repo, got error: %v", err)
	}
	if foundRoot != childDir {
		t.Errorf("Expected root %s, got %s", childDir, foundRoot)
	}
	
	// Search from parent (where boundary is) should NOT find grandparent
	_, err = FindSourceRepoRoot(parentDir)
	if err != ErrRootNotFound {
		t.Errorf("Expected ErrRootNotFound at boundary, got: %v", err)
	}
}
