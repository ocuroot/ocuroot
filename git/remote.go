package git

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// GitObject represents a file in a git repository
type GitObject struct {
	// Content is the file contents as bytes
	Content []byte
	// Tombstone indicates this file should be deleted
	Tombstone bool
}

type RemoteGit interface {
	Endpoint() string
	BranchRefs(ctx context.Context) ([]Ref, error)
	TagRefs(ctx context.Context) ([]Ref, error)

	GetTree(ctx context.Context, ref string) (*TreeNode, error)
	GetObject(ctx context.Context, hash string) ([]byte, error)
	GetCommitMessage(ctx context.Context, hash string) (string, error)

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

func (t *TreeNode) Paths() []string {
	if t == nil {
		return []string{}
	}

	var paths []string
	if t.IsObject {
		return []string{""}
	}
	for subPath, k := range t.Children {
		childPaths := k.Paths()
		for _, childPath := range childPaths {
			if childPath == "" {
				paths = append(paths, subPath)
			} else {
				paths = append(paths, subPath+"/"+childPath)
			}
		}
	}
	return paths
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
			// Object was not found
			return nil, nil
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
