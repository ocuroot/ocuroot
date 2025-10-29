package refstore

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
)

// Global mutex map to ensure only one operation per worktree
var (
	worktreeMutexes   = make(map[string]*sync.Mutex)
	worktreeMutexesMu sync.Mutex
)

func getWorktreeMutex(worktreePath string) *sync.Mutex {
	worktreeMutexesMu.Lock()
	defer worktreeMutexesMu.Unlock()
	
	if mu, exists := worktreeMutexes[worktreePath]; exists {
		return mu
	}
	
	mu := &sync.Mutex{}
	worktreeMutexes[worktreePath] = mu
	return mu
}

// NewGitBackend creates a new git backend using a local bare repository with worktrees.
// bareRepoPath: path to the bare repository (will be created if it doesn't exist)
// remoteURL: the remote repository URL to fetch from and push to
// branch: the branch name (without refs/heads/ prefix)
// gitUserName: git author name (optional, defaults to system config)
// gitUserEmail: git author email (optional, defaults to system config)
func NewGitBackend(ctx context.Context, bareRepoPath string, remoteURL string, branch string, gitUserName string, gitUserEmail string) (DocumentBackend, error) {
	// Ensure branch doesn't have refs/heads/ prefix for worktree operations
	branch = strings.TrimPrefix(branch, "refs/heads/")
	
	// Initialize bare repo if it doesn't exist
	if err := initBareRepo(bareRepoPath, remoteURL); err != nil {
		return nil, fmt.Errorf("failed to init bare repo: %w", err)
	}
	
	// Fetch from remote to ensure we have latest refs
	if err := fetchRemote(bareRepoPath); err != nil {
		return nil, fmt.Errorf("failed to fetch remote: %w", err)
	}
	
	// Create worktree for this branch
	worktreePath := filepath.Join(bareRepoPath, "worktrees", branch)
	if err := ensureWorktree(bareRepoPath, worktreePath, branch); err != nil {
		return nil, fmt.Errorf("failed to ensure worktree: %w", err)
	}

	return &gitBackend{
		bareRepoPath: bareRepoPath,
		worktreePath: worktreePath,
		remoteURL:    remoteURL,
		branch:       branch,
		gitUserName:  gitUserName,
		gitUserEmail: gitUserEmail,
	}, nil
}

var _ DocumentBackend = (*gitBackend)(nil)

type gitBackend struct {
	bareRepoPath string
	worktreePath string
	remoteURL    string
	branch       string
	gitUserName  string
	gitUserEmail string
}

// GetBytes implements DocumentBackend.
func (g *gitBackend) GetBytes(ctx context.Context, path string) ([]byte, error) {
	filePath := filepath.Join(g.worktreePath, path)
	
	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read file %s: %w", path, err)
	}

	return data, nil
}

// SetBytes implements DocumentBackend.
func (g *gitBackend) SetBytes(ctx context.Context, path string, content []byte) error {
	// Use mutex to prevent concurrent operations on the same worktree
	mu := getWorktreeMutex(g.worktreePath)
	mu.Lock()
	defer mu.Unlock()

	// Pull latest changes from remote
	if err := g.pullWorktree(); err != nil {
		return fmt.Errorf("failed to pull: %w", err)
	}

	// Write directly to file
	filePath := filepath.Join(g.worktreePath, path)
	if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}
	
	if err := os.WriteFile(filePath, content, 0644); err != nil {
		return fmt.Errorf("failed to write file %s: %w", path, err)
	}

	// Commit and push
	if err := g.gitAdd(); err != nil {
		return fmt.Errorf("failed to add: %w", err)
	}

	if err := g.gitCommit(fmt.Sprintf("Update %s", path)); err != nil {
		return fmt.Errorf("failed to commit: %w", err)
	}

	if err := g.gitPush(); err != nil {
		return fmt.Errorf("failed to push: %w", err)
	}

	return nil
}

// Get implements DocumentBackend.
func (g *gitBackend) Get(ctx context.Context, refs []string) ([]GetResult, error) {
	var out []GetResult

	for _, ref := range refs {
		filePath := filepath.Join(g.worktreePath, ref)
		
		// Check if file exists
		data, err := os.ReadFile(filePath)
		if err != nil {
			if os.IsNotExist(err) {
				// File doesn't exist, return empty result
				out = append(out, GetResult{
					Path: ref,
				})
				continue
			}
			return nil, fmt.Errorf("failed to read %s: %w", ref, err)
		}

		// Unmarshal the document
		var doc StorageObject
		if err := json.Unmarshal(data, &doc); err != nil {
			return nil, fmt.Errorf("failed to unmarshal %s: %w", ref, err)
		}

		out = append(out, GetResult{
			Path: ref,
			Doc:  &doc,
		})
	}
	
	return out, nil
}

// Marker implements DocumentBackend.
func (g *gitBackend) Marker() ([]byte, error) {
	panic("unimplemented")
}

// Match implements DocumentBackend.
func (g *gitBackend) Match(ctx context.Context, reqs []MatchRequest) ([]string, error) {
	compiledReqs, err := compileMatchRequests(reqs)
	if err != nil {
		return nil, err
	}

	var out []string
	for _, req := range compiledReqs {
		searchPath := g.worktreePath
		if req.prefix != "" {
			searchPath = filepath.Join(g.worktreePath, req.prefix)
		}

		// Check if search path exists
		if _, err := os.Stat(searchPath); os.IsNotExist(err) {
			continue
		}

		var suffixes = req.suffixes
		if len(suffixes) == 0 {
			suffixes = []string{""}
		}

		// Walk the directory tree
		err := filepath.Walk(searchPath, func(filePath string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() {
				return nil
			}

			// Get relative path from worktree root
			relPath, err := filepath.Rel(g.worktreePath, filePath)
			if err != nil {
				return err
			}

			// Get relative path from search path
			relFromPrefix, err := filepath.Rel(searchPath, filePath)
			if err != nil {
				return err
			}

			// Check suffixes and glob
			for _, suffix := range suffixes {
				if strings.HasSuffix(relFromPrefix, suffix) && req.compiledGlob.Match(strings.TrimSuffix(relFromPrefix, suffix)) {
					out = append(out, filepath.ToSlash(relPath))
					break
				}
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}

	return out, nil
}

// Set implements DocumentBackend.
func (g *gitBackend) Set(ctx context.Context, marker []byte, message string, reqs []SetRequest) error {
	// Use mutex to prevent concurrent operations on the same worktree
	mu := getWorktreeMutex(g.worktreePath)
	mu.Lock()
	defer mu.Unlock()

	// Pull latest changes from remote
	if err := g.pullWorktree(); err != nil {
		return fmt.Errorf("failed to pull: %w", err)
	}

	// Apply changes to worktree
	for _, req := range reqs {
		filePath := filepath.Join(g.worktreePath, req.Path)
		
		if req.Doc == nil {
			// Delete file
			if err := os.Remove(filePath); err != nil && !os.IsNotExist(err) {
				return fmt.Errorf("failed to delete %s: %w", req.Path, err)
			}
		} else {
			// Write file
			docContent, err := json.Marshal(req.Doc)
			if err != nil {
				return fmt.Errorf("failed to marshal doc: %w", err)
			}
			
			// Ensure directory exists
			if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
				return fmt.Errorf("failed to create directory: %w", err)
			}
			
			if err := os.WriteFile(filePath, docContent, 0644); err != nil {
				return fmt.Errorf("failed to write file: %w", err)
			}
		}
	}

	// Stage all changes
	if err := g.gitAdd(); err != nil {
		return fmt.Errorf("failed to stage changes: %w", err)
	}

	// Commit changes
	if err := g.gitCommit(message); err != nil {
		return fmt.Errorf("failed to commit: %w", err)
	}

	// Push to remote
	if err := g.gitPush(); err != nil {
		return fmt.Errorf("failed to push: %w", err)
	}

	return nil
}

// Helper functions for git operations

func initBareRepo(bareRepoPath string, remoteURL string) error {
	// Check if bare repo already exists
	if _, err := os.Stat(filepath.Join(bareRepoPath, "config")); err == nil {
		return nil // Already initialized
	}

	// Create directory
	if err := os.MkdirAll(bareRepoPath, 0755); err != nil {
		return err
	}

	// Initialize bare repo
	cmd := exec.Command("git", "init", "--bare", bareRepoPath)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git init failed: %w: %s", err, output)
	}

	// Add remote
	cmd = exec.Command("git", "-C", bareRepoPath, "remote", "add", "origin", remoteURL)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git remote add failed: %w: %s", err, output)
	}

	return nil
}

func fetchRemote(bareRepoPath string) error {
	cmd := exec.Command("git", "-C", bareRepoPath, "fetch", "origin")
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git fetch failed: %w: %s", err, output)
	}
	return nil
}

func ensureWorktree(bareRepoPath string, worktreePath string, branch string) error {
	// Check if worktree already exists
	if _, err := os.Stat(filepath.Join(worktreePath, ".git")); err == nil {
		return nil // Already exists
	}

	// Get absolute paths
	absBareRepoPath, err := filepath.Abs(bareRepoPath)
	if err != nil {
		return err
	}
	absWorktreePath, err := filepath.Abs(worktreePath)
	if err != nil {
		return err
	}

	// Create worktree directory parent
	if err := os.MkdirAll(filepath.Dir(absWorktreePath), 0755); err != nil {
		return err
	}

	// Try to create worktree from existing remote branch
	cmd := exec.Command("git", "-C", absBareRepoPath, "worktree", "add", absWorktreePath, "-b", branch, "origin/"+branch)
	if err := cmd.Run(); err != nil {
		// If branch doesn't exist on remote, create orphan branch
		cmd = exec.Command("git", "-C", absBareRepoPath, "worktree", "add", "--orphan", "-b", branch, absWorktreePath)
		if output, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("git worktree add --orphan failed: %w: %s", err, output)
		}

		// Create initial commit in the worktree
		cmd = exec.Command("git", "-C", absWorktreePath, "commit", "--allow-empty", "-m", "Initial commit")
		if output, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("git commit failed: %w: %s", err, output)
		}

		// Push to create branch on remote
		cmd = exec.Command("git", "-C", absWorktreePath, "push", "-u", "origin", branch)
		if output, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("git push failed: %w: %s", err, output)
		}
	}

	return nil
}

func (g *gitBackend) pullWorktree() error {
	// Check if there are any local changes
	cmd := exec.Command("git", "-C", g.worktreePath, "diff", "--quiet")
	hasChanges := cmd.Run() != nil
	
	cmd = exec.Command("git", "-C", g.worktreePath, "diff", "--cached", "--quiet")
	hasStagedChanges := cmd.Run() != nil
	
	if hasChanges || hasStagedChanges {
		// If there are local changes, skip the pull
		// The changes will be committed and pushed in this Set operation
		return nil
	}
	
	cmd = exec.Command("git", "-C", g.worktreePath, "pull", "--rebase")
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git pull failed: %w: %s", err, output)
	}
	return nil
}

func (g *gitBackend) gitAdd() error {
	cmd := exec.Command("git", "-C", g.worktreePath, "add", "-A")
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git add failed: %w: %s", err, output)
	}
	return nil
}

func (g *gitBackend) gitCommit(message string) error {
	// Check if there are changes to commit
	cmd := exec.Command("git", "-C", g.worktreePath, "diff", "--cached", "--quiet")
	if err := cmd.Run(); err == nil {
		// No changes to commit
		return nil
	}

	// Use a default message if empty
	if message == "" {
		message = "Update"
	}

	// Build commit command with author if provided
	args := []string{"-C", g.worktreePath, "commit", "-m", message}
	if g.gitUserName != "" && g.gitUserEmail != "" {
		args = append(args, "--author", fmt.Sprintf("%s <%s>", g.gitUserName, g.gitUserEmail))
	}

	cmd = exec.Command("git", args...)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git commit failed: %w: %s", err, output)
	}
	return nil
}

func (g *gitBackend) gitPush() error {
	cmd := exec.Command("git", "-C", g.worktreePath, "push")
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git push failed: %w: %s", err, output)
	}
	return nil
}
