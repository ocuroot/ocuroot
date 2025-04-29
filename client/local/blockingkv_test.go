package local

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"pgregory.net/rapid"
)

func TestBlockingKVStoreSetBeforeGet(t *testing.T) {
	store := NewBlockingKVStore[string, string]()
	store.Set("test", "value")
	value, ok := store.Get("test")
	if !ok {
		t.Fatal("expected value")
	}
	if value != "value" {
		t.Fatalf("expected value 'value', got '%s'", value)
	}
}

func TestBlockingKVStoreGetBeforeSet(t *testing.T) {
	store := NewBlockingKVStore[string, string]()

	ch := make(chan error)

	go func() {
		value, ok := store.Get("test")
		if !ok {
			ch <- fmt.Errorf("expected value")
		}
		if value != "value" {
			ch <- fmt.Errorf("expected value 'value', got '%s'", value)
		}

		close(ch)
	}()

	store.Set("test", "value")

	select {
	case err := <-ch:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("timeout")
	}
}

func TestBlockingKVStoreUnset(t *testing.T) {
	store := NewBlockingKVStore[string, string]()
	store.Set("test", "value")
	store.Unset("test")
	value, ok := store.Get("test")
	if ok {
		t.Fatal("expected no value")
	}
	if value != "" {
		t.Fatalf("expected empty value, got '%s'", value)
	}
}

func TestBlockingKVStoreRapid(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		store := NewBlockingKVStore[int, bool]()

		values := rapid.SliceOfNDistinct(rapid.Int(), 1, 10, func(i int) int {
			return i
		}).Draw(t, "values")

		var wg sync.WaitGroup
		wg.Add(len(values))

		for _, v := range values {
			go func(v int) {
				defer wg.Done()
				value, ok := store.Get(v)
				if !ok {
					t.Fatalf("expected a value for key %d", v)
				}
				if value != true {
					t.Fatalf("expected true value for key %d", v)
				}
			}(v)
			store.Set(v, true)
		}
		wg.Wait()
	})
}
