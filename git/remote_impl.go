package git

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"path"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/log"
	"github.com/go-git/go-git/v6/plumbing"
	"github.com/go-git/go-git/v6/plumbing/filemode"
	"github.com/go-git/go-git/v6/plumbing/format/packfile"
	"github.com/go-git/go-git/v6/plumbing/object"
	"github.com/go-git/go-git/v6/plumbing/protocol/packp"
	"github.com/go-git/go-git/v6/plumbing/protocol/packp/capability"
	"github.com/go-git/go-git/v6/plumbing/transport"
	"github.com/go-git/go-git/v6/storage/memory"

	_ "github.com/go-git/go-git/v6/plumbing/transport/file"
	_ "github.com/go-git/go-git/v6/plumbing/transport/git"
	_ "github.com/go-git/go-git/v6/plumbing/transport/http"
	_ "github.com/go-git/go-git/v6/plumbing/transport/ssh"
)

func NewRemoteGit(endpoint string) (RemoteGit, error) {
	return NewRemoteGitWithUser(endpoint, nil)
}

func NewRemoteGitWithUser(endpoint string, user *GitUser) (RemoteGit, error) {
	ep, err := transport.NewEndpoint(endpoint)
	if err != nil {
		return nil, err
	}

	log.Info("Endpoint", "endpoint", endpoint, "protocol", ep.Protocol)

	c, err := transport.Get(ep.Protocol)
	if err != nil {
		return nil, err
	}

	store := memory.NewStorage()

	return &remoteGitImpl{
		endpoint:  endpoint,
		transport: c,
		ep:        ep,
		store:     store,
		user:      user,
	}, nil
}

type remoteGitImpl struct {
	endpoint string

	transport transport.Transport
	ep        *transport.Endpoint
	store     *memory.Storage
	user      *GitUser

	// Cached connection for reuse
	cachedConn transport.Connection
}

func (r *remoteGitImpl) Endpoint() string {
	return r.endpoint
}

// getOrCreateConnection returns a valid connection, reusing the cached one if available
// or creating a new one if needed. The caller should NOT close the returned connection
// as it may be reused. Use invalidateConnection() if the connection fails.
func (r *remoteGitImpl) getOrCreateConnection(ctx context.Context) (transport.Connection, error) {
	// Try to reuse cached connection if it exists
	if r.cachedConn != nil {
		return r.cachedConn, nil
	}

	// Create new session and connection
	sess, err := r.transport.NewSession(r.store, r.ep, nil)
	if err != nil {
		return nil, err
	}

	conn, err := sess.Handshake(ctx, transport.UploadPackService, "")
	if err != nil {
		return nil, err
	}

	// Cache the connection for reuse
	r.cachedConn = conn
	return conn, nil
}

// InvalidateConnection closes and clears the cached connection
func (r *remoteGitImpl) InvalidateConnection() {
	if r.cachedConn != nil {
		r.cachedConn.Close()
		r.cachedConn = nil
	}
}

// invalidateConnection is a private alias for backwards compatibility
func (r *remoteGitImpl) invalidateConnection() {
	r.InvalidateConnection()
}

// BranchRefs implements RemoteGit.
func (r *remoteGitImpl) BranchRefs(ctx context.Context) ([]Ref, error) {
	conn, err := r.getOrCreateConnection(ctx)
	if err != nil {
		log.Error("Failed to get connection", "error", err)
		return nil, err
	}

	refs, err := conn.GetRemoteRefs(ctx)
	if err != nil {
		// Connection might be stale, invalidate and retry once
		r.invalidateConnection()
		conn, err = r.getOrCreateConnection(ctx)
		if err != nil {
			log.Error("Failed to get connection on retry", "error", err)
			return nil, err
		}
		refs, err = conn.GetRemoteRefs(ctx)
		if err != nil {
			// Empty repositories should just return empty refs
			if errors.Is(err, transport.ErrEmptyRemoteRepository) {
				return nil, nil
			}
			log.Error("Failed to get remote refs on retry", "error", err)
			return nil, err
		}
	}

	var out []Ref
	for _, ref := range refs {
		if ref.Name().IsBranch() {
			out = append(out, Ref{
				Name: ref.Name().String(),
				Hash: ref.Hash().String(),
			})
		}
	}

	return out, nil
}

// GetTree implements RemoteGit.
func (r *remoteGitImpl) GetTree(ctx context.Context, hash string) (*TreeNode, error) {
	hashObj, ok := plumbing.FromHex(hash)
	if !ok {
		return nil, fmt.Errorf("invalid hash: %v, got %v", hash, hashObj)
	}

	conn, err := r.getOrCreateConnection(ctx)
	if err != nil {
		return nil, err
	}

	// Build fetch request
	fetchReq := &transport.FetchRequest{
		Wants: []plumbing.Hash{hashObj},
		Depth: 0,
	}

	// Only add filter if the server supports it
	caps := conn.Capabilities()
	if caps.Supports(capability.Filter) {
		fetchReq.Filter = packp.FilterBlobLimit(0, packp.BlobLimitPrefixNone)
	}

	err = conn.Fetch(ctx, fetchReq)
	if err != nil && !strings.Contains(err.Error(), "empty packfile") {
		// Connection might be stale, invalidate and retry once
		// Ignore "empty packfile" errors - means we already have everything
		r.invalidateConnection()
		conn, err = r.getOrCreateConnection(ctx)
		if err != nil {
			return nil, err
		}

		// Rebuild fetch request with new connection's capabilities
		caps = conn.Capabilities()
		if caps.Supports(capability.Filter) {
			fetchReq.Filter = packp.FilterBlobLimit(0, packp.BlobLimitPrefixNone)
		} else {
			fetchReq.Filter = ""
		}

		err = conn.Fetch(ctx, fetchReq)
		if err != nil && !strings.Contains(err.Error(), "empty packfile") {
			return nil, err
		}
	}

	// Get the commit object
	commit, err := r.store.EncodedObject(plumbing.CommitObject, hashObj)
	if err != nil {
		return nil, fmt.Errorf("failed to get commit object: %w", err)
	}

	// Decode the commit to get the tree hash
	commitDecoded, err := object.DecodeCommit(r.store, commit)
	if err != nil {
		return nil, fmt.Errorf("failed to decode commit: %w", err)
	}

	// Get the tree object
	treeHash := commitDecoded.TreeHash
	treeObj, err := r.store.EncodedObject(plumbing.TreeObject, treeHash)
	if err != nil {
		return nil, fmt.Errorf("failed to get tree object: %w", err)
	}

	// Decode and build the tree structure
	tree, err := object.DecodeTree(r.store, treeObj)
	if err != nil {
		return nil, fmt.Errorf("failed to decode tree: %w", err)
	}

	return r.buildTreeNode(tree)
}

// buildTreeNode recursively builds a TreeNode from an object.Tree
func (r *remoteGitImpl) buildTreeNode(tree *object.Tree) (*TreeNode, error) {
	node := &TreeNode{
		Hash:     tree.Hash.String(),
		IsObject: false,
		Children: make(map[string]*TreeNode),
	}

	for _, entry := range tree.Entries {
		if entry.Mode.IsFile() {
			// It's a blob (file)
			node.Children[entry.Name] = &TreeNode{
				Hash:     entry.Hash.String(),
				IsObject: true,
				Children: nil,
			}
		} else if entry.Mode == filemode.Dir {
			// It's a subtree (directory)
			subTreeObj, err := r.store.EncodedObject(plumbing.TreeObject, entry.Hash)
			if err != nil {
				return nil, fmt.Errorf("failed to get subtree %s: %w", entry.Name, err)
			}

			subTree, err := object.DecodeTree(r.store, subTreeObj)
			if err != nil {
				return nil, fmt.Errorf("failed to decode subtree %s: %w", entry.Name, err)
			}

			subNode, err := r.buildTreeNode(subTree)
			if err != nil {
				return nil, fmt.Errorf("failed to build subtree %s: %w", entry.Name, err)
			}

			node.Children[entry.Name] = subNode
		}
		// Skip other modes (symlinks, submodules, etc.) for now
	}

	return node, nil
}

// GetObject implements RemoteGit.
func (r *remoteGitImpl) GetObject(ctx context.Context, hash string) ([]byte, error) {
	hashObj, ok := plumbing.FromHex(hash)
	if !ok {
		return nil, fmt.Errorf("invalid hash: %v", hash)
	}

	// Try to get the blob from storage first
	blob, err := r.store.EncodedObject(plumbing.BlobObject, hashObj)
	if err != nil {
		// If not in storage, fetch it
		conn, err := r.getOrCreateConnection(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to get connection: %w", err)
		}

		err = conn.Fetch(ctx, &transport.FetchRequest{
			Wants: []plumbing.Hash{hashObj},
			Depth: 0,
		})
		if err != nil && !strings.Contains(err.Error(), "empty packfile") {
			// Connection might be stale, invalidate and retry once
			// Ignore "empty packfile" errors - means we already have everything
			r.invalidateConnection()
			conn, err = r.getOrCreateConnection(ctx)
			if err != nil {
				return nil, fmt.Errorf("failed to get connection on retry: %w", err)
			}

			err = conn.Fetch(ctx, &transport.FetchRequest{
				Wants: []plumbing.Hash{hashObj},
				Depth: 0,
			})
			if err != nil && !strings.Contains(err.Error(), "empty packfile") {
				return nil, fmt.Errorf("failed to fetch blob: %w", err)
			}
		}

		// Try to get it again after fetching
		blob, err = r.store.EncodedObject(plumbing.BlobObject, hashObj)
		if err != nil {
			return nil, fmt.Errorf("failed to get blob object after fetch: %w", err)
		}
	}

	// Read the blob contents
	reader, err := blob.Reader()
	if err != nil {
		return nil, fmt.Errorf("failed to get blob reader: %w", err)
	}
	defer reader.Close()

	// Handle empty files
	if blob.Size() == 0 {
		return []byte{}, nil
	}

	// Read all content
	content := make([]byte, blob.Size())
	_, err = reader.Read(content)
	if err != nil && err.Error() != "EOF" {
		return nil, fmt.Errorf("failed to read blob content: %w", err)
	}

	return content, nil
}

// GetCommitMessage implements RemoteGit.
func (r *remoteGitImpl) GetCommitMessage(ctx context.Context, hash string) (string, error) {
	hashObj, ok := plumbing.FromHex(hash)
	if !ok {
		return "", fmt.Errorf("invalid hash: %v", hash)
	}

	// Try to get the commit from storage first
	commit, err := r.store.EncodedObject(plumbing.CommitObject, hashObj)
	if err != nil {
		// If not in storage, fetch it
		conn, err := r.getOrCreateConnection(ctx)
		if err != nil {
			return "", fmt.Errorf("failed to get connection: %w", err)
		}

		err = conn.Fetch(ctx, &transport.FetchRequest{
			Wants: []plumbing.Hash{hashObj},
			Depth: 0,
		})
		if err != nil && !strings.Contains(err.Error(), "empty packfile") {
			// Connection might be stale, invalidate and retry once
			// Ignore "empty packfile" errors - means we already have everything
			r.invalidateConnection()
			conn, err = r.getOrCreateConnection(ctx)
			if err != nil {
				return "", fmt.Errorf("failed to get connection on retry: %w", err)
			}

			err = conn.Fetch(ctx, &transport.FetchRequest{
				Wants: []plumbing.Hash{hashObj},
				Depth: 0,
			})
			if err != nil && !strings.Contains(err.Error(), "empty packfile") {
				return "", fmt.Errorf("failed to fetch commit: %w", err)
			}
		}

		// Try to get it again after fetching
		commit, err = r.store.EncodedObject(plumbing.CommitObject, hashObj)
		if err != nil {
			return "", fmt.Errorf("failed to get commit object after fetch: %w", err)
		}
	}

	// Decode the commit to get the message
	commitDecoded, err := object.DecodeCommit(r.store, commit)
	if err != nil {
		return "", fmt.Errorf("failed to decode commit: %w", err)
	}

	return commitDecoded.Message, nil
}

// Push implements RemoteGit.
func (r *remoteGitImpl) Push(ctx context.Context, refName string, objectsByPath map[string]GitObject, message string) error {
	return r.pushWithOptions(ctx, refName, objectsByPath, message, true)
}

// pushWithStaleRef is an internal helper for testing race conditions
// It pushes using a specific old hash without fetching current refs
func (r *remoteGitImpl) pushWithStaleRef(ctx context.Context, refName string, objectsByPath map[string]GitObject, message string, staleOldHash plumbing.Hash) error {
	// Build the tree from the objects
	rootTreeHash, err := r.buildTreeFromObjects(ctx, refName, objectsByPath, staleOldHash)
	if err != nil {
		return fmt.Errorf("failed to build tree: %w", err)
	}

	if r.user == nil {
		return errors.New("user must be set to push")
	}

	// Validate user
	if err := r.user.Validate(); err != nil {
		return fmt.Errorf("invalid user: %w", err)
	}

	// Use the provided stale hash as both old hash and parent
	oldHash := staleOldHash
	var parentHashes []plumbing.Hash
	if !staleOldHash.IsZero() {
		parentHashes = []plumbing.Hash{staleOldHash}
	}

	// Create a commit object with the stale parent
	commit := &object.Commit{
		Author: object.Signature{
			Name:  r.user.Name,
			Email: r.user.Email,
			When:  time.Now(),
		},
		Committer: object.Signature{
			Name:  r.user.Name,
			Email: r.user.Email,
			When:  time.Now(),
		},
		Message:      message,
		TreeHash:     rootTreeHash,
		ParentHashes: parentHashes,
	}

	// Encode and store the commit
	commitObj := r.store.NewEncodedObject()
	commitObj.SetType(plumbing.CommitObject)
	if err := commit.Encode(commitObj); err != nil {
		return fmt.Errorf("failed to encode commit: %w", err)
	}

	commitHash, err := r.store.SetEncodedObject(commitObj)
	if err != nil {
		return fmt.Errorf("failed to store commit: %w", err)
	}

	// Build push command with the stale old hash
	cmd := &packp.Command{
		Name: plumbing.ReferenceName(refName),
		Old:  oldHash,
		New:  commitHash,
	}

	// Create packfile with all objects
	packfileReader, err := r.createPackfile(commitHash)
	if err != nil {
		return fmt.Errorf("failed to create packfile: %w", err)
	}

	// Create a new session for the push
	sess, err := r.transport.NewSession(r.store, r.ep, nil)
	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}

	conn, err := sess.Handshake(ctx, transport.ReceivePackService, "")
	if err != nil {
		return fmt.Errorf("failed to handshake: %w", err)
	}
	defer conn.Close()

	// Build push request
	pushReq := &transport.PushRequest{
		Packfile: packfileReader,
		Commands: []*packp.Command{cmd},
	}

	// Push to remote
	err = conn.Push(ctx, pushReq)
	if err != nil {
		return fmt.Errorf("failed to push: %w", err)
	}

	// Invalidate cached connection since push changes remote state
	// This ensures subsequent operations see the updated refs
	r.invalidateConnection()

	return nil
}

// pushWithOptions is an internal helper that allows controlling whether to fetch fresh refs
// This is used for testing race conditions
func (r *remoteGitImpl) pushWithOptions(ctx context.Context, refName string, objectsByPath map[string]GitObject, message string, fetchFreshRefs bool) error {
	var oldHash plumbing.Hash = plumbing.ZeroHash
	var parentHashes []plumbing.Hash

	if fetchFreshRefs {
		// Invalidate any cached connection to ensure we get fresh refs
		// This is critical for detecting race conditions
		r.invalidateConnection()
	}

	// Create a new session for receive-pack to get current refs
	sess, err := r.transport.NewSession(r.store, r.ep, nil)
	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}

	conn, err := sess.Handshake(ctx, transport.ReceivePackService, "")
	if err != nil {
		return fmt.Errorf("failed to handshake: %w", err)
	}
	defer conn.Close()

	// Get current refs to determine parent commit and old hash
	refs, err := conn.GetRemoteRefs(ctx)
	if err != nil {
		return fmt.Errorf("failed to get remote refs: %w", err)
	}

	for _, ref := range refs {
		if ref.Name().String() == refName {
			oldHash = ref.Hash()
			// Set this commit as the parent of our new commit
			parentHashes = []plumbing.Hash{oldHash}
			break
		}
	}

	// Build the tree from the objects
	rootTreeHash, err := r.buildTreeFromObjects(ctx, refName, objectsByPath, oldHash)
	if err != nil {
		return fmt.Errorf("failed to build tree: %w", err)
	}

	if r.user == nil {
		return errors.New("user must be set to push")
	}

	// Validate user
	if err := r.user.Validate(); err != nil {
		return fmt.Errorf("invalid user: %w", err)
	}

	// Create a commit object with proper parent
	commit := &object.Commit{
		Author: object.Signature{
			Name:  r.user.Name,
			Email: r.user.Email,
			When:  time.Now(),
		},
		Committer: object.Signature{
			Name:  r.user.Name,
			Email: r.user.Email,
			When:  time.Now(),
		},
		Message:      message,
		TreeHash:     rootTreeHash,
		ParentHashes: parentHashes, // Set parent to current HEAD
	}

	// Encode and store the commit
	commitObj := r.store.NewEncodedObject()
	commitObj.SetType(plumbing.CommitObject)
	if err := commit.Encode(commitObj); err != nil {
		return fmt.Errorf("failed to encode commit: %w", err)
	}

	commitHash, err := r.store.SetEncodedObject(commitObj)
	if err != nil {
		return fmt.Errorf("failed to store commit: %w", err)
	}

	// Build push command
	cmd := &packp.Command{
		Name: plumbing.ReferenceName(refName),
		Old:  oldHash,
		New:  commitHash,
	}

	// Create packfile with all objects
	packfileReader, err := r.createPackfile(commitHash)
	if err != nil {
		return fmt.Errorf("failed to create packfile: %w", err)
	}

	// Build push request
	pushReq := &transport.PushRequest{
		Packfile: packfileReader,
		Commands: []*packp.Command{cmd},
	}

	// Push to remote
	err = conn.Push(ctx, pushReq)
	if err != nil {
		return fmt.Errorf("failed to push: %w", err)
	}

	// Invalidate cached connection since push changes remote state
	// This ensures subsequent operations see the updated refs
	r.invalidateConnection()

	return nil
}

// dirEntry represents an entry in a directory (file or subdirectory)
type dirEntry struct {
	name   string
	hash   plumbing.Hash
	isTree bool
}

// buildTreeFromObjects creates a tree structure from a map of file paths to GitObjects
// It handles both file creation/updates and deletions (via tombstones)
// This merges changes with the parent tree without fetching blob contents
func (r *remoteGitImpl) buildTreeFromObjects(ctx context.Context, refName string, objectsByPath map[string]GitObject, parentHash plumbing.Hash) (plumbing.Hash, error) {
	// Start with a map of file paths to blob hashes from the parent
	fileHashes := make(map[string]plumbing.Hash)
	
	if !parentHash.IsZero() {
		// Get the parent tree structure
		// The parent commit must already be in our store - we don't fetch it here
		// to avoid goroutine explosion and packfile corruption issues
		commit, err := r.store.EncodedObject(plumbing.CommitObject, parentHash)
		if err != nil {
			// Parent commit not in store - we can't preserve files from it
			// This is expected when using separate RemoteGit instances
			// Just start with an empty tree
			log.Warn("Parent commit not in local store, cannot preserve existing files", "parent", parentHash.String())
		} else {
			commitDecoded, err := object.DecodeCommit(r.store, commit)
			if err != nil {
				return plumbing.ZeroHash, fmt.Errorf("failed to decode parent commit: %w", err)
			}

			treeObj, err := r.store.EncodedObject(plumbing.TreeObject, commitDecoded.TreeHash)
			if err != nil {
				return plumbing.ZeroHash, fmt.Errorf("failed to get parent tree: %w", err)
			}

			parentTree, err := object.DecodeTree(r.store, treeObj)
			if err != nil {
				return plumbing.ZeroHash, fmt.Errorf("failed to decode parent tree: %w", err)
			}

			// Collect file paths and their blob hashes (without reading content)
			if err := r.collectBlobHashes("", parentTree, fileHashes); err != nil {
				return plumbing.ZeroHash, fmt.Errorf("failed to collect blob hashes: %w", err)
			}
		}
	}

	// Apply changes: create new blobs for changed files, remove deleted files
	for filePath, obj := range objectsByPath {
		if obj.Tombstone {
			// Delete the file
			delete(fileHashes, filePath)
		} else {
			// Create a new blob for this file
			blob := r.store.NewEncodedObject()
			blob.SetType(plumbing.BlobObject)
			writer, err := blob.Writer()
			if err != nil {
				return plumbing.ZeroHash, fmt.Errorf("failed to create blob writer: %w", err)
			}

			if _, err := writer.Write(obj.Content); err != nil {
				writer.Close()
				return plumbing.ZeroHash, fmt.Errorf("failed to write blob: %w", err)
			}
			writer.Close()

			blobHash, err := r.store.SetEncodedObject(blob)
			if err != nil {
				return plumbing.ZeroHash, fmt.Errorf("failed to store blob: %w", err)
			}

			fileHashes[filePath] = blobHash
		}
	}

	// Build the tree structure from the file hashes
	return r.buildTreeFromHashes(fileHashes)
}

// collectBlobHashes recursively collects file paths and their blob hashes from a tree
func (r *remoteGitImpl) collectBlobHashes(dirPath string, tree *object.Tree, hashes map[string]plumbing.Hash) error {
	for _, entry := range tree.Entries {
		fullPath := path.Join(dirPath, entry.Name)
		if dirPath == "" {
			fullPath = entry.Name
		}

		if entry.Mode == filemode.Dir {
			// Recursively collect from subtree
			subTreeObj, err := r.store.EncodedObject(plumbing.TreeObject, entry.Hash)
			if err != nil {
				return fmt.Errorf("failed to get subtree %s: %w", entry.Name, err)
			}

			subTree, err := object.DecodeTree(r.store, subTreeObj)
			if err != nil {
				return fmt.Errorf("failed to decode subtree %s: %w", entry.Name, err)
			}

			if err := r.collectBlobHashes(fullPath, subTree, hashes); err != nil {
				return err
			}
		} else if entry.Mode.IsFile() {
			// Store the blob hash (don't read the content)
			hashes[fullPath] = entry.Hash
		}
	}

	return nil
}

// buildTreeFromHashes builds a tree structure from a map of file paths to blob hashes
func (r *remoteGitImpl) buildTreeFromHashes(fileHashes map[string]plumbing.Hash) (plumbing.Hash, error) {
	// Group files by directory
	dirs := make(map[string][]dirEntry)

	for filePath, blobHash := range fileHashes {
		dir := path.Dir(filePath)
		if dir == "." {
			dir = ""
		}
		fileName := path.Base(filePath)

		dirs[dir] = append(dirs[dir], dirEntry{
			name:   fileName,
			hash:   blobHash,
			isTree: false,
		})
	}

	// Build trees bottom-up
	return r.buildTree("", dirs)
}

// buildTree recursively builds tree objects
func (r *remoteGitImpl) buildTree(dirPath string, dirs map[string][]dirEntry) (plumbing.Hash, error) {
	tree := &object.Tree{}

	// Add entries from this directory
	entries := dirs[dirPath]
	for _, entry := range entries {
		mode := filemode.Regular
		if entry.isTree {
			mode = filemode.Dir
		}

		tree.Entries = append(tree.Entries, object.TreeEntry{
			Name: entry.name,
			Mode: mode,
			Hash: entry.hash,
		})
	}

	// Find subdirectories and build their trees
	prefix := dirPath
	if prefix != "" {
		prefix += "/"
	}

	subdirs := make(map[string]bool)
	for dir := range dirs {
		if dir != dirPath && strings.HasPrefix(dir, prefix) {
			// Find immediate subdirectory
			remainder := strings.TrimPrefix(dir, prefix)
			subdir := strings.Split(remainder, "/")[0]
			subdirs[subdir] = true
		}
	}

	// Build subtrees
	for subdir := range subdirs {
		subdirPath := prefix + subdir
		if dirPath == "" {
			subdirPath = subdir
		}

		subtreeHash, err := r.buildTree(subdirPath, dirs)
		if err != nil {
			return plumbing.ZeroHash, err
		}

		tree.Entries = append(tree.Entries, object.TreeEntry{
			Name: subdir,
			Mode: filemode.Dir,
			Hash: subtreeHash,
		})
	}

	// Sort entries (required by git)
	sort.Slice(tree.Entries, func(i, j int) bool {
		return tree.Entries[i].Name < tree.Entries[j].Name
	})

	// Encode and store the tree
	treeObj := r.store.NewEncodedObject()
	treeObj.SetType(plumbing.TreeObject)
	if err := tree.Encode(treeObj); err != nil {
		return plumbing.ZeroHash, fmt.Errorf("failed to encode tree: %w", err)
	}

	treeHash, err := r.store.SetEncodedObject(treeObj)
	if err != nil {
		return plumbing.ZeroHash, fmt.Errorf("failed to store tree: %w", err)
	}

	return treeHash, nil
}

// createPackfile creates a packfile containing all objects needed for the commit
func (r *remoteGitImpl) createPackfile(commitHash plumbing.Hash) (io.ReadCloser, error) {
	// Use go-git's packfile builder
	buf := new(bytes.Buffer)
	encoder := packfile.NewEncoder(buf, r.store, false)

	// Get all objects reachable from the commit
	// We collect hashes as we walk, ensuring all objects are in the store
	var hashes []plumbing.Hash
	
	// Add the commit itself
	hashes = append(hashes, commitHash)

	// Walk the commit to find all referenced objects
	commit, err := object.GetCommit(r.store, commitHash)
	if err != nil {
		return nil, fmt.Errorf("failed to get commit: %w", err)
	}

	// Recursively collect all tree and blob hashes
	if err := r.collectTreeHashes(commit.TreeHash, &hashes); err != nil {
		return nil, fmt.Errorf("failed to collect tree hashes: %w", err)
	}

	// Check if we have any objects to encode
	if len(hashes) == 0 {
		return nil, fmt.Errorf("no objects to encode in packfile")
	}

	// Encode all objects into the packfile
	if _, err := encoder.Encode(hashes, 0); err != nil {
		return nil, fmt.Errorf("failed to encode packfile: %w", err)
	}

	return io.NopCloser(bytes.NewReader(buf.Bytes())), nil
}

// collectTreeHashes recursively collects all tree and blob hashes reachable from a tree
func (r *remoteGitImpl) collectTreeHashes(treeHash plumbing.Hash, hashes *[]plumbing.Hash) error {
	// Add the tree itself
	*hashes = append(*hashes, treeHash)

	// Get the tree object
	treeObj, err := r.store.EncodedObject(plumbing.TreeObject, treeHash)
	if err != nil {
		return fmt.Errorf("failed to get tree %s: %w", treeHash, err)
	}

	tree, err := object.DecodeTree(r.store, treeObj)
	if err != nil {
		return fmt.Errorf("failed to decode tree %s: %w", treeHash, err)
	}

	// Walk through all entries
	for _, entry := range tree.Entries {
		if entry.Mode == filemode.Dir {
			// Recursively collect subtree hashes
			if err := r.collectTreeHashes(entry.Hash, hashes); err != nil {
				return err
			}
		} else if entry.Mode.IsFile() {
			// Add blob hash
			*hashes = append(*hashes, entry.Hash)
		}
	}

	return nil
}

// TagRefs implements RemoteGit.
func (r *remoteGitImpl) TagRefs(ctx context.Context) ([]Ref, error) {
	conn, err := r.getOrCreateConnection(ctx)
	if err != nil {
		return nil, err
	}

	refs, err := conn.GetRemoteRefs(ctx)
	if err != nil {
		// Connection might be stale, invalidate and retry once
		r.invalidateConnection()
		conn, err = r.getOrCreateConnection(ctx)
		if err != nil {
			return nil, err
		}
		refs, err = conn.GetRemoteRefs(ctx)
		if err != nil {
			// Empty repositories should just return empty refs
			if errors.Is(err, transport.ErrEmptyRemoteRepository) {
				return nil, nil
			}
			return nil, err
		}
	}

	var out []Ref
	for _, ref := range refs {
		if ref.Name().IsTag() {
			out = append(out, Ref{
				Name: ref.Name().String(),
				Hash: ref.Hash().String(),
			})
		}
	}

	return out, nil
}

// CreateBranch implements RemoteGit.
// Creates a new branch on the remote repository without checking it out.
// If sourceRef is empty, creates an orphan branch with an empty initial commit.
// If sourceRef is provided, it can be a commit hash or a branch reference name.
func (r *remoteGitImpl) CreateBranch(ctx context.Context, branchName string, sourceRef string, message string) error {
	if r.user == nil {
		return errors.New("user must be set to create branch")
	}

	// Validate user
	if err := r.user.Validate(); err != nil {
		return fmt.Errorf("invalid user: %w", err)
	}

	// Ensure branch name is in refs/heads/ format
	refName := branchName
	if !strings.HasPrefix(refName, "refs/heads/") {
		refName = "refs/heads/" + branchName
	}

	// Create a new session for receive-pack
	sess, err := r.transport.NewSession(r.store, r.ep, nil)
	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}

	conn, err := sess.Handshake(ctx, transport.ReceivePackService, "")
	if err != nil {
		return fmt.Errorf("failed to handshake: %w", err)
	}
	defer conn.Close()

	// Get current refs to check if branch already exists
	refs, err := conn.GetRemoteRefs(ctx)
	if err != nil {
		return fmt.Errorf("failed to get remote refs: %w", err)
	}

	// Check if branch already exists
	for _, ref := range refs {
		if ref.Name().String() == refName {
			return fmt.Errorf("branch %s already exists", branchName)
		}
	}

	var commitHash plumbing.Hash

	if sourceRef == "" {
		// Create an orphan branch with an empty tree
		emptyTree := &object.Tree{}
		treeObj := r.store.NewEncodedObject()
		treeObj.SetType(plumbing.TreeObject)
		if err := emptyTree.Encode(treeObj); err != nil {
			return fmt.Errorf("failed to encode empty tree: %w", err)
		}

		treeHash, err := r.store.SetEncodedObject(treeObj)
		if err != nil {
			return fmt.Errorf("failed to store empty tree: %w", err)
		}

		// Create initial commit with empty tree and no parents
		commit := &object.Commit{
			Author: object.Signature{
				Name:  r.user.Name,
				Email: r.user.Email,
				When:  time.Now(),
			},
			Committer: object.Signature{
				Name:  r.user.Name,
				Email: r.user.Email,
				When:  time.Now(),
			},
			Message:      message,
			TreeHash:     treeHash,
			ParentHashes: nil, // No parents for orphan branch
		}

		commitObj := r.store.NewEncodedObject()
		commitObj.SetType(plumbing.CommitObject)
		if err := commit.Encode(commitObj); err != nil {
			return fmt.Errorf("failed to encode commit: %w", err)
		}

		commitHash, err = r.store.SetEncodedObject(commitObj)
		if err != nil {
			return fmt.Errorf("failed to store commit: %w", err)
		}
	} else {
		// Resolve sourceRef to a commit hash
		var sourceHash plumbing.Hash

		// Try to parse as a hash first
		if h, ok := plumbing.FromHex(sourceRef); ok {
			sourceHash = h
		} else {
			// Try to find it as a ref name
			sourceRefName := sourceRef
			if !strings.HasPrefix(sourceRefName, "refs/") {
				sourceRefName = "refs/heads/" + sourceRef
			}

			found := false
			for _, ref := range refs {
				if ref.Name().String() == sourceRefName {
					sourceHash = ref.Hash()
					found = true
					break
				}
			}

			if !found {
				return fmt.Errorf("source ref %s not found", sourceRef)
			}
		}

		// Fetch the source commit to ensure we have it
		fetchReq := &transport.FetchRequest{
			Wants: []plumbing.Hash{sourceHash},
			Depth: 0,
		}

		// Create a new connection for fetching
		fetchConn, err := r.getOrCreateConnection(ctx)
		if err != nil {
			return fmt.Errorf("failed to get connection for fetch: %w", err)
		}

		err = fetchConn.Fetch(ctx, fetchReq)
		if err != nil {
			return fmt.Errorf("failed to fetch source commit: %w", err)
		}

		// The new branch will point to the same commit
		commitHash = sourceHash
	}

	// Build push command to create the new branch
	cmd := &packp.Command{
		Name: plumbing.ReferenceName(refName),
		Old:  plumbing.ZeroHash, // Creating a new ref
		New:  commitHash,
	}

	// Create packfile with all objects
	packfileReader, err := r.createPackfile(commitHash)
	if err != nil {
		return fmt.Errorf("failed to create packfile: %w", err)
	}

	// Build push request
	pushReq := &transport.PushRequest{
		Packfile: packfileReader,
		Commands: []*packp.Command{cmd},
	}

	// Push to remote
	err = conn.Push(ctx, pushReq)
	if err != nil {
		return fmt.Errorf("failed to push branch: %w", err)
	}

	// Invalidate cached connection since push changes remote state
	r.invalidateConnection()

	return nil
}
