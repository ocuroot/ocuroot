package git

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

type RemoteGit interface {
	BranchRefs(ctx context.Context) ([]Ref, error)
	TagRefs(ctx context.Context) ([]Ref, error)

	GetTree(ctx context.Context, ref string) (*TreeNode, error)
	GetObject(ctx context.Context, hash string) ([]byte, error)
	Push(ctx context.Context, refName string, objectsByPath map[string]string, message string) error
	
	// CreateBranch creates a new branch on the remote repository without checking it out.
	// The branch is created from the specified sourceRef (commit hash or branch name).
	// If sourceRef is empty, creates an orphan branch with an empty initial commit.
	// This works on newly-initialized bare repos as well as repos with existing branches.
	CreateBranch(ctx context.Context, branchName string, sourceRef string, message string) error
	
	// InvalidateConnection closes and clears any cached connection
	// This can be useful for polling scenarios to ensure fresh data
	InvalidateConnection()
}

// GitUser represents a git user for commits
type GitUser struct {
	Name  string
	Email string
}

// Validate checks if the GitUser has required fields
func (u *GitUser) Validate() error {
	if u.Name == "" {
		return fmt.Errorf("git user name is required")
	}
	if u.Email == "" {
		return fmt.Errorf("git user email is required")
	}
	return nil
}

type Ref struct {
	Name string
	Hash string
}

type TreeNode struct {
	Hash     string
	IsObject bool
	Children map[string]*TreeNode
}

func (t *TreeNode) NodeAtPath(p string) (*TreeNode, error) {
	if t == nil {
		return nil, errors.New("tree is nil")
	}

	if t.Hash == "" {
		return nil, errors.New("tree hash is empty")
	}

	if p == "" || p == "/" {
		return t, nil
	}

	// Split path into components
	parts := strings.Split(strings.Trim(p, "/"), "/")
	
	current := t
	for i, part := range parts {
		if part == "" {
			continue
		}
		
		child, ok := current.Children[part]
		if !ok {
			return nil, fmt.Errorf("object not found: %s", p)
		}
		
		// If this is the last part, return it
		if i == len(parts)-1 {
			return child, nil
		}
		
		// Otherwise, it should be a directory to continue traversing
		if child.IsObject {
			return nil, fmt.Errorf("path component %s is a file, not a directory", part)
		}
		
		current = child
	}

	return current, nil
}
