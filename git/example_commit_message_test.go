package git_test

import (
	"context"
	"fmt"
	"log"

	"github.com/ocuroot/ocuroot/git"
)

// Example demonstrates how to use GetCommitMessage to retrieve commit messages from a remote repository
func ExampleRemoteGit_GetCommitMessage() {
	ctx := context.Background()

	// Connect to a remote repository
	remote, err := git.NewRemoteGit("https://github.com/user/repo.git")
	if err != nil {
		log.Fatal(err)
	}

	// Get the branch refs to find a commit hash
	refs, err := remote.BranchRefs(ctx)
	if err != nil {
		log.Fatal(err)
	}

	if len(refs) == 0 {
		log.Fatal("No branches found")
	}

	// Get the commit message for the main branch's HEAD
	commitHash := refs[0].Hash
	message, err := remote.GetCommitMessage(ctx, commitHash)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Commit %s message:\n%s\n", commitHash[:8], message)
}
