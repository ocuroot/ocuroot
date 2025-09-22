package refstore

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/ocuroot/gittools"
)

func TestGitRefStore(t *testing.T) {
	tempDir := "./testdata/git_testdata"
	_ = os.RemoveAll(tempDir)
	if err := os.MkdirAll(tempDir, os.ModePerm); err != nil {
		t.Fatal(err)
	}

	remotePath, cleanup, err := gittools.CreateTestRemoteRepoWithBranch("ocuroot_test", "main")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(cleanup)

	localPath := filepath.Join(tempDir, "local_repo")

	store, err := NewGitRefStore(
		localPath,
		remotePath,
		"store",
		GitRefStoreConfig{
			PathPrefix: "subpath",
			GitRepoConfig: GitRepoConfig{
				CreateBranch: true,
			},
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := store.Close(); err != nil {
			t.Error(err)
		}
	}()

	DoTestStore(t, store)
}

func TestGitRefStoreWithTransaction(t *testing.T) {
	CheckStagedFiles = true

	tempDir, err := os.MkdirTemp("", "ocuroot_test")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		os.RemoveAll(tempDir)
	})

	remotePath, cleanup, err := gittools.CreateTestRemoteRepo("ocuroot_test")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(cleanup)

	localPath := filepath.Join(tempDir, "local_repo")

	store, err := NewGitRefStore(localPath, remotePath, "main", GitRefStoreConfig{
		GitRepoConfig: GitRepoConfig{
			CreateBranch: true,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := store.Close(); err != nil {
			t.Error(err)
		}
	}()

	if err := store.StartTransaction(context.Background(), "test"); err != nil {
		t.Fatal(err)
	}
	DoTestStore(t, store)
	if err := store.CommitTransaction(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestGitRefStoreTransaction(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "ocuroot_test")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		os.RemoveAll(tempDir)
	})

	remotePath, cleanup, err := gittools.CreateTestRemoteRepo("ocuroot_test")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(cleanup)

	localPath := filepath.Join(tempDir, "local_repo")

	store, err := NewGitRefStore(localPath, remotePath, "main", GitRefStoreConfig{
		GitRepoConfig: GitRepoConfig{
			CreateBranch: true,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := store.Close(); err != nil {
			t.Error(err)
		}
	}()

	ctx := context.Background()

	if err := store.StartTransaction(ctx, "test"); err != nil {
		t.Fatal(err)
	}

	if err := store.Set(ctx, "test", "test"); err != nil {
		t.Fatal(err)
	}

	var got string
	if err := store.Get(ctx, "test", &got); err != nil {
		t.Fatal(err)
	}
	if got != "test" {
		t.Fatalf("expected test, got %s", got)
	}

	if err := store.CommitTransaction(ctx); err != nil {
		t.Fatal(err)
	}

	if err := store.Get(ctx, "test", &got); err != nil {
		t.Fatal(err)
	}
	if got != "test" {
		t.Fatalf("expected test, got %s", got)
	}
}

func TestGitRefStoreAddSupportFiles(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "ocuroot_test")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		os.RemoveAll(tempDir)
	})

	remotePath, cleanup, err := gittools.CreateTestRemoteRepo("ocuroot_test")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(cleanup)

	localPath := filepath.Join(tempDir, "local_repo")

	store, err := NewGitRefStore(localPath, remotePath, "main", GitRefStoreConfig{
		GitRepoConfig: GitRepoConfig{
			CreateBranch: true,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := store.Close(); err != nil {
			t.Error(err)
		}
	}()

	ctx := context.Background()

	if err := store.AddSupportFiles(ctx, map[string]string{
		".github/workflows/ocuroot.yml": "name: ocuroot\n",
	}); err != nil {
		t.Fatal(err)
	}

	checkoutTempDir, err := os.MkdirTemp("", "ocuroot_test")
	if err != nil {
		t.Fatal(err)
	}
	if false {
		t.Cleanup(func() {
			os.RemoveAll(checkoutTempDir)
		})
	}
	_, err = gittools.NewClient().Clone(remotePath, checkoutTempDir)
	if err != nil {
		t.Fatal(err)
	}

	filePath := filepath.Join(checkoutTempDir, ".github", "workflows", "ocuroot.yml")
	if _, err := os.Stat(filePath); err != nil {
		t.Fatalf("expected file %s to exist, but got error %s", filePath, err)
	}

}
