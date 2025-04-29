package refstore

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/ocuroot/gittools"
)

func TestGitRefStore(t *testing.T) {
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

	store, err := NewGitRefStore(localPath, remotePath, "master")
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

	store, err := NewGitRefStore(localPath, remotePath, "master")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := store.Close(); err != nil {
			t.Error(err)
		}
	}()

	if err := store.StartTransaction(context.Background()); err != nil {
		t.Fatal(err)
	}
	DoTestStore(t, store)
	if err := store.CommitTransaction(context.Background(), "test"); err != nil {
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

	store, err := NewGitRefStore(localPath, remotePath, "master")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := store.Close(); err != nil {
			t.Error(err)
		}
	}()

	ctx := context.Background()

	if err := store.StartTransaction(ctx); err != nil {
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

	if err := store.CommitTransaction(ctx, "test"); err != nil {
		t.Fatal(err)
	}

	if err := store.Get(ctx, "test", &got); err != nil {
		t.Fatal(err)
	}
	if got != "test" {
		t.Fatalf("expected test, got %s", got)
	}
}
