package refstore

import (
	"context"
	"encoding/json"
	"path"
	"strings"

	"github.com/ocuroot/ocuroot/git"
)

func NewGitBackend(ctx context.Context, remote git.RemoteGit, branch string) (DocumentBackend, error) {
	if !strings.HasPrefix(branch, "refs/") {
		branch = "refs/heads/" + branch
	}

	brs, err := remote.BranchRefs(ctx)
	if err != nil {
		return nil, err
	}

	var branchExists bool
	for _, br := range brs {
		if br.Name == branch {
			branchExists = true
			break
		}
	}

	if !branchExists {
		err = remote.CreateBranch(ctx, branch, "", "Creating branch")
		if err != nil {
			return nil, err
		}
	}

	return &gitBackend{
		remote: remote,
		branch: branch,
	}, nil
}

var _ DocumentBackend = (*gitBackend)(nil)

type gitBackend struct {
	remote git.RemoteGit
	branch string
}

// GetInfo implements DocumentBackend.
func (g *gitBackend) GetInfo(ctx context.Context) (*StoreInfo, error) {
	results, err := g.Get(ctx, []string{storeInfoFile})
	if err != nil {
		return nil, err
	}
	if len(results) != 1 {
		return nil, nil
	}
	if results[0].Doc == nil {
		return nil, nil
	}

	var info StoreInfo
	if err := json.Unmarshal(results[0].Doc.Body, &info); err != nil {
		return nil, err
	}
	return &info, nil
}

// SetInfo implements DocumentBackend.
func (g *gitBackend) SetInfo(ctx context.Context, info *StoreInfo) error {
	content, err := json.Marshal(info)
	if err != nil {
		return err
	}

	return g.Set(ctx, nil, "Update store info", []SetRequest{
		{
			Path: storeInfoFile,
			Doc: &StorageObject{
				Body: content,
			},
		},
	})
}

// Get implements DocumentBackend.
func (g *gitBackend) Get(ctx context.Context, refs []string) ([]GetResult, error) {
	brs, err := g.remote.BranchRefs(ctx)
	if err != nil {
		return nil, err
	}

	var tree *git.TreeNode
	for _, br := range brs {
		if br.Name == g.branch {
			tree, err = g.remote.GetTree(ctx, br.Hash)
			if err != nil {
				return nil, err
			}
			break
		}
	}

	var out []GetResult

	// If the branch does not exist, return an empty set of results
	if tree == nil {
		for _, ref := range refs {
			out = append(out, GetResult{
				Path: ref,
			})
		}
		return out, nil
	}

	for _, ref := range refs {
		n, err := tree.NodeAtPath(ref)
		if err != nil {
			return nil, err
		}
		if n != nil && n.IsObject {
			body, err := g.remote.GetObject(ctx, n.Hash)
			if err != nil {
				return nil, err
			}
			var doc StorageObject
			if err := json.Unmarshal(body, &doc); err != nil {
				return nil, err
			}

			out = append(out, GetResult{
				Path: ref,
				Doc:  &doc,
			})
		} else {
			out = append(out, GetResult{
				Path: ref,
			})
		}
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

	brs, err := g.remote.BranchRefs(ctx)
	if err != nil {
		return nil, err
	}

	var tree *git.TreeNode
	for _, br := range brs {
		if br.Name == g.branch {
			tree, err = g.remote.GetTree(ctx, br.Hash)
			if err != nil {
				return nil, err
			}
			break
		}
	}

	if tree == nil {
		return nil, nil
	}

	var out []string
	for _, req := range compiledReqs {
		t := tree
		if req.prefix != "" {
			t, err = tree.NodeAtPath(req.prefix)
			if err != nil {
				return nil, err
			}
		}

		if t == nil {
			continue
		}

		var suffixes = req.suffixes
		if len(suffixes) == 0 {
			suffixes = []string{""}
		}

		for _, p := range t.Paths() {
			for _, suffix := range suffixes {
				if strings.HasSuffix(p, suffix) && req.compiledGlob.Match(strings.TrimSuffix(p, suffix)) {
					out = append(out, path.Join(req.prefix, p))
				}
			}
		}

	}

	return out, nil
}

// Set implements DocumentBackend.
func (g *gitBackend) Set(ctx context.Context, marker []byte, message string, reqs []SetRequest) error {
	var objects = make(map[string]git.GitObject)
	for _, req := range reqs {
		if req.Doc == nil {
			// Tombstone for deletion
			objects[req.Path] = git.GitObject{
				Tombstone: true,
			}
		} else {
			docContent, err := json.Marshal(req.Doc)
			if err != nil {
				return err
			}
			objects[req.Path] = git.GitObject{
				Content: docContent,
			}
		}
	}

	return g.remote.Push(ctx, g.branch, objects, message)

}
