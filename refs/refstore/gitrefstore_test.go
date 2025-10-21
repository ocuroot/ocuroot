package refstore

import "testing"

func TestGitRefStore(t *testing.T) {
	// TODO: This tests passes, but takes 17s for some reason!
	t.Skip()

	be, err := NewGitBackend(t.Context(), remoteWithBareRepoAtDir(t, "git/refstore.git"), "main")
	if err != nil {
		t.Fatal(err)
	}
	store, err := NewRefStore(t.Context(), be, map[string]struct{}{})
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
