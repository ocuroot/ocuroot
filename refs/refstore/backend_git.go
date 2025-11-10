package refstore

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/oklog/ulid/v2"
)

// No global mutexes needed - each backend instance has its own clone

// NewGitBackend creates a new git backend using a unique clone of the repository.
// bareRepoPath: path to the bare repository (will be created if it doesn't exist)
// remoteURL: the remote repository URL to fetch from and push to
// branch: the branch name (without refs/heads/ prefix)
// pathPrefix: optional path prefix to scope all operations (e.g., "state" or "intent")
// gitUserName: git author name (optional, defaults to system config)
// gitUserEmail: git author email (optional, defaults to system config)
func NewGitBackend(ctx context.Context, bareRepoPath string, remoteURL string, branch string, pathPrefix string, gitUserName string, gitUserEmail string) (DocumentBackend, error) {
	// Ensure branch doesn't have refs/heads/ prefix
	branch = strings.TrimPrefix(branch, "refs/heads/")
	
	// Validate branch name
	if branch == "" {
		return nil, fmt.Errorf("branch name cannot be empty")
	}
	
	// Initialize bare repo if it doesn't exist
	if err := initBareRepo(bareRepoPath, remoteURL); err != nil {
		return nil, fmt.Errorf("initializing bare repo: %w", err)
	}
	
	// Fetch from remote to ensure we have latest refs
	if err := fetchRemote(bareRepoPath); err != nil {
		return nil, fmt.Errorf("fetching remote: %w", err)
	}
	
	// Create a unique clone directory using ULID to avoid collisions
	cloneID := ulid.Make().String()
	clonesBaseDir := filepath.Join(filepath.Dir(bareRepoPath), "clones")
	clonePath := filepath.Join(clonesBaseDir, cloneID)
	
	if err := cloneRepo(remoteURL, clonePath, branch); err != nil {
		return nil, fmt.Errorf("cloning repo: %w", err)
	}

	return &gitBackend{
		bareRepoPath: bareRepoPath,
		clonePath:    clonePath,
		remoteURL:    remoteURL,
		branch:       branch,
		pathPrefix:   pathPrefix,
		gitUserName:  gitUserName,
		gitUserEmail: gitUserEmail,
	}, nil
}

var _ DocumentBackend = (*gitBackend)(nil)

type gitBackend struct {
	bareRepoPath string
	clonePath    string
	remoteURL    string
	branch       string
	pathPrefix   string
	gitUserName  string
	gitUserEmail string
}

// GetBytes implements DocumentBackend.
func (g *gitBackend) GetBytes(ctx context.Context, path string) ([]byte, error) {
	filePath := filepath.Join(g.clonePath, g.pathPrefix, path)
	
	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading file %s: %w", path, err)
	}

	return data, nil
}

// SetBytes implements DocumentBackend.
func (g *gitBackend) SetBytes(ctx context.Context, path string, content []byte) error {
	return g.setBytesWithPrefix(ctx, path, content, true)
}

// setBytesWithPrefix writes a file with optional path prefix
func (g *gitBackend) setBytesWithPrefix(ctx context.Context, path string, content []byte, usePrefix bool) error {
	// Pull latest changes from remote
	if err := g.pullClone(); err != nil {
		return fmt.Errorf("pulling: %w", err)
	}

	// Write directly to file
	var filePath string
	if usePrefix {
		filePath = filepath.Join(g.clonePath, g.pathPrefix, path)
	} else {
		filePath = filepath.Join(g.clonePath, path)
	}
	
	if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
		return fmt.Errorf("creating directory: %w", err)
	}
	
	if err := os.WriteFile(filePath, content, 0644); err != nil {
		return fmt.Errorf("writing file %s: %w", path, err)
	}

	// Commit and push
	if err := g.gitAdd(); err != nil {
		return fmt.Errorf("adding: %w", err)
	}

	if err := g.gitCommit(fmt.Sprintf("Update %s", path)); err != nil {
		return fmt.Errorf("committing: %w", err)
	}

	if err := g.gitPush(); err != nil {
		return fmt.Errorf("pushing: %w", err)
	}

	return nil
}

// SetBytesAtRoot writes a file at the repository root, ignoring the path prefix
func (g *gitBackend) SetBytesAtRoot(ctx context.Context, path string, content []byte) error {
	return g.setBytesWithPrefix(ctx, path, content, false)
}

// Get implements DocumentBackend.
func (g *gitBackend) Get(ctx context.Context, refs []string) ([]GetResult, error) {
	var out []GetResult

	for _, ref := range refs {
		filePath := filepath.Join(g.clonePath, g.pathPrefix, ref)
		
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
			return nil, fmt.Errorf("reading %s: %w", ref, err)
		}

		// Unmarshal the document
		var doc StorageObject
		if err := json.Unmarshal(data, &doc); err != nil {
			return nil, fmt.Errorf("unmarshaling %s: %w", ref, err)
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
		searchPath := filepath.Join(g.clonePath, g.pathPrefix)
		if req.prefix != "" {
			searchPath = filepath.Join(g.clonePath, g.pathPrefix, req.prefix)
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

			// Get relative path from pathPrefix root
			relPath, err := filepath.Rel(filepath.Join(g.clonePath, g.pathPrefix), filePath)
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
	// Pull latest changes from remote
	if err := g.pullClone(); err != nil {
		return fmt.Errorf("pulling: %w", err)
	}

	// Apply changes to worktree
	for _, req := range reqs {
		filePath := filepath.Join(g.clonePath, g.pathPrefix, req.Path)
		
		if req.Doc == nil {
			// Delete file
			if err := os.Remove(filePath); err != nil && !os.IsNotExist(err) {
				return fmt.Errorf("deleting %s: %w", req.Path, err)
			}
		} else {
			// Write file
			docContent, err := json.Marshal(req.Doc)
			if err != nil {
				return fmt.Errorf("marshaling doc: %w", err)
			}
			
			// Ensure directory exists
			if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
				return fmt.Errorf("creating directory: %w", err)
			}
			
			if err := os.WriteFile(filePath, docContent, 0644); err != nil {
				return fmt.Errorf("writing file: %w", err)
			}
		}
	}

	// Stage all changes
	if err := g.gitAdd(); err != nil {
		return fmt.Errorf("staging changes: %w", err)
	}

	// Commit changes
	if err := g.gitCommit(message); err != nil {
		return fmt.Errorf("committing: %w", err)
	}

	// Push to remote
	if err := g.gitPush(); err != nil {
		return fmt.Errorf("pushing: %w", err)
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
	// Use --force to handle cases where local refs are out of sync with remote
	// This can happen in test scenarios or when multiple processes access the repo
	cmd := exec.Command("git", "-C", bareRepoPath, "fetch", "--force", "origin")
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git fetch failed: %w: %s", err, output)
	}
	return nil
}

func cloneRepo(remoteURL string, clonePath string, branch string) error {
	// Create clone directory parent
	if err := os.MkdirAll(filepath.Dir(clonePath), 0755); err != nil {
		return fmt.Errorf("creating clone directory: %w", err)
	}

	// Try to clone the specific branch
	cmd := exec.Command("git", "clone", "--branch", branch, "--single-branch", remoteURL, clonePath)
	if _, err := cmd.CombinedOutput(); err != nil {
		// Branch doesn't exist, try cloning without specifying a branch
		cmd = exec.Command("git", "clone", remoteURL, clonePath)
		if _, err := cmd.CombinedOutput(); err != nil {
			// Empty bare repo, initialize a new clone manually
			if err := os.MkdirAll(clonePath, 0755); err != nil {
				return fmt.Errorf("creating clone directory: %w", err)
			}
			
			// Initialize new repo
			cmd = exec.Command("git", "-C", clonePath, "init")
			if output, err := cmd.CombinedOutput(); err != nil {
				return fmt.Errorf("git init failed: %w: %s", err, output)
			}
			
			// Add remote
			cmd = exec.Command("git", "-C", clonePath, "remote", "add", "origin", remoteURL)
			if output, err := cmd.CombinedOutput(); err != nil {
				return fmt.Errorf("git remote add failed: %w: %s", err, output)
			}
		}

		// Create and checkout orphan branch
		cmd = exec.Command("git", "-C", clonePath, "checkout", "--orphan", branch)
		if output, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("git checkout --orphan failed: %w: %s", err, output)
		}

		// Remove all files from index
		cmd = exec.Command("git", "-C", clonePath, "rm", "-rf", ".")
		cmd.Run() // Ignore error if there are no files

		// Make initial commit
		cmd = exec.Command("git", "-C", clonePath, "commit", "--allow-empty", "-m", "Initial commit")
		if output, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("git commit failed: %w: %s", err, output)
		}

		// Push to create branch on remote
		cmd = exec.Command("git", "-C", clonePath, "push", "-u", "origin", branch)
		if output, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("git push failed: %w: %s", err, output)
		}
	}

	return nil
}

func (g *gitBackend) pullClone() error {
	// Check if there are any unstaged changes
	cmd := exec.Command("git", "-C", g.clonePath, "diff", "--quiet")
	hasUnstagedChanges := cmd.Run() != nil
	
	// Check if there are any staged changes
	cmd = exec.Command("git", "-C", g.clonePath, "diff", "--cached", "--quiet")
	hasStagedChanges := cmd.Run() != nil
	
	if hasUnstagedChanges || hasStagedChanges {
		// If there are local changes, skip the pull
		// The changes will be committed and pushed in this Set operation
		return nil
	}
	
	// Use --autostash to handle any unexpected staged changes
	// This can happen if there's a race between the check above and the pull
	cmd = exec.Command("git", "-C", g.clonePath, "pull", "--rebase", "--autostash")
	output, err := cmd.CombinedOutput()
	if err != nil {
		// Check if this is a rebase conflict
		outputStr := string(output)
		if strings.Contains(outputStr, "CONFLICT") {
			// Auto-resolve conflicts by accepting our version (--ours)
			// This is safe when multiple workers are writing identical or compatible content
			
			// Use checkout --ours to resolve all conflicts in favor of our changes
			checkoutCmd := exec.Command("git", "-C", g.clonePath, "checkout", "--ours", ".")
			if checkoutErr := checkoutCmd.Run(); checkoutErr != nil {
				// If that fails, abort the rebase
				abortCmd := exec.Command("git", "-C", g.clonePath, "rebase", "--abort")
				abortCmd.Run()
				return fmt.Errorf("git checkout --ours failed: %w", checkoutErr)
			}
			
			// Stage the resolved files
			addCmd := exec.Command("git", "-C", g.clonePath, "add", "-A")
			if addErr := addCmd.Run(); addErr != nil {
				abortCmd := exec.Command("git", "-C", g.clonePath, "rebase", "--abort")
				abortCmd.Run()
				return fmt.Errorf("git add after conflict resolution failed: %w", addErr)
			}
			
			// Continue the rebase
			continueCmd := exec.Command("git", "-C", g.clonePath, "rebase", "--continue")
			continueCmd.Env = append(os.Environ(), "GIT_EDITOR=true") // Skip commit message editing
			if continueOutput, continueErr := continueCmd.CombinedOutput(); continueErr != nil {
				// Check if it's just "no changes" which is fine
				if !strings.Contains(string(continueOutput), "No changes") {
					abortCmd := exec.Command("git", "-C", g.clonePath, "rebase", "--abort")
					abortCmd.Run()
					return fmt.Errorf("git rebase --continue failed: %w: %s", continueErr, continueOutput)
				}
				// If no changes, skip this commit
				skipCmd := exec.Command("git", "-C", g.clonePath, "rebase", "--skip")
				if skipErr := skipCmd.Run(); skipErr != nil {
					abortCmd := exec.Command("git", "-C", g.clonePath, "rebase", "--abort")
					abortCmd.Run()
					return fmt.Errorf("git rebase --skip failed: %w", skipErr)
				}
			}
			
			return nil
		}
		
		return fmt.Errorf("git pull failed: %w: %s", err, output)
	}
	return nil
}

func (g *gitBackend) gitAdd() error {
	// Add files, being careful to only add data files and not worktree metadata
	// Strategy: explicitly add files that should be tracked, avoiding metadata files
	
	if g.pathPrefix != "" {
		// When using a pathPrefix, add both:
		// 1. Files in the pathPrefix directory (data files)
		// 2. Files at the root (support files like .gitignore, support.txt, etc.)
		
		// First, add files from the pathPrefix directory if it exists
		prefixFullPath := filepath.Join(g.clonePath, g.pathPrefix)
		if _, err := os.Stat(prefixFullPath); err == nil {
			cmd := exec.Command("git", "-C", g.clonePath, "add", "-A", g.pathPrefix)
			if output, err := cmd.CombinedOutput(); err != nil {
				return fmt.Errorf("git add failed: %w: %s", err, output)
			}
		}
		
		// Then, add any root-level files (but not directories, to avoid metadata)
		// List files in the root directory
		entries, err := os.ReadDir(g.clonePath)
		if err != nil {
			return fmt.Errorf("reading worktree: %w", err)
		}
		
		for _, entry := range entries {
			// Skip directories and .git
			if entry.IsDir() || entry.Name() == ".git" {
				continue
			}
			
			// Add this root-level file
			cmd := exec.Command("git", "-C", g.clonePath, "add", entry.Name())
			if output, err := cmd.CombinedOutput(); err != nil {
				return fmt.Errorf("git add %s failed: %w: %s", entry.Name(), err, output)
			}
		}
	} else {
		// No pathPrefix, add everything from current directory
		cmd := exec.Command("git", "-C", g.clonePath, "add", "-A", ".")
		if output, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("git add failed: %w: %s", err, output)
		}
	}
	
	return nil
}

func (g *gitBackend) gitCommit(message string) error {
	// Check if there are changes to commit
	cmd := exec.Command("git", "-C", g.clonePath, "diff", "--cached", "--quiet")
	if err := cmd.Run(); err == nil {
		// No changes to commit
		return nil
	}

	// Use a default message if empty
	if message == "" {
		message = "Update"
	}

	// Build commit command with author if provided
	args := []string{"-C", g.clonePath, "commit", "-m", message}
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
	// Retry push with pull-rebase if it fails due to remote changes
	maxRetries := 3
	for attempt := 0; attempt < maxRetries; attempt++ {
		cmd := exec.Command("git", "-C", g.clonePath, "push")
		output, err := cmd.CombinedOutput()
		if err == nil {
			return nil
		}
		
		// Check if the error is due to remote changes
		outputStr := string(output)
		if strings.Contains(outputStr, "rejected") || strings.Contains(outputStr, "fetch first") || strings.Contains(outputStr, "failed to update ref") {
			// Pull with rebase and retry
			if attempt < maxRetries-1 {
				if err := g.pullClone(); err != nil {
					return fmt.Errorf("pull before retry failed: %w", err)
				}
				continue
			}
		}
		
		return fmt.Errorf("git push failed: %w: %s", err, output)
	}
	return fmt.Errorf("git push failed after %d retries", maxRetries)
}

// Close cleans up the clone directory
func (g *gitBackend) Close() error {
	if g.clonePath != "" {
		if err := os.RemoveAll(g.clonePath); err != nil {
			return fmt.Errorf("removing clone directory: %w", err)
		}
	}
	return nil
}
