package client

import (
	"github.com/ocuroot/ocuroot/refs"
	"github.com/ocuroot/ocuroot/refs/refstore"
)

func New(wd string) (*Client, error) {
	// TODO: Handle being in a state repo or directory

	repoRoot, err := FindRepoRoot(wd)
	if err != nil {
		return nil, err
	}

	repoURL, commit, err := GetRepoInfo(repoRoot)
	if err != nil {
		return nil, err
	}

	return &Client{
		RepoInfo: &RepoInfo{
			Root:   repoRoot,
			Commit: commit,
			RepoRef: refs.Ref{
				Repo: repoURL,
			},
		},
	}, nil
}

type Client struct {
	RepoInfo *RepoInfo

	State refstore.Store
}

type RepoInfo struct {
	Root    string
	Commit  string
	RepoRef refs.Ref
}

func (c *Client) InRepo() bool {
	return c.RepoInfo != nil
}

/*
We may need separate clients for managing state,
handling releases and similar.

Functions to include:
* NewRelease
* ContinueRelease
* ApproveRelease
* LocalRelease
* Preview
*/
