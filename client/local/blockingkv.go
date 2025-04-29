package local

import "sync"

type KVStore[T comparable, K any] interface {
	Waiting() []T
	Unset(name T)
	Set(name T, value K)
	Get(name T) (K, bool)
}

var _ KVStore[string, string] = (*BlockingKVStore[string, string])(nil)

func NewBlockingKVStore[T comparable, K any]() *BlockingKVStore[T, K] {
	return &BlockingKVStore[T, K]{
		values:      map[T]K{},
		unsetValues: map[T]struct{}{},
		valuesSet:   map[T]chan struct{}{},
	}
}

type BlockingKVStore[T comparable, K any] struct {
	mtx sync.Mutex

	values      map[T]K
	unsetValues map[T]struct{}
	valuesSet   map[T]chan struct{}
}

func (b *BlockingKVStore[T, K]) Waiting() []T {
	if b == nil {
		return nil
	}

	b.mtx.Lock()
	defer b.mtx.Unlock()
	var names []T
	for name := range b.valuesSet {
		select {
		case <-b.valuesSet[name]:
		default:
			names = append(names, name)
		}
	}
	return names
}

func (b *BlockingKVStore[T, K]) Unset(name T) {
	if b == nil {
		return
	}

	b.mtx.Lock()
	defer b.mtx.Unlock()
	b.unsetValues[name] = struct{}{}

	if _, exists := b.valuesSet[name]; !exists {
		b.valuesSet[name] = make(chan struct{})
	}
	select {
	case <-b.valuesSet[name]:
	default:
		close(b.valuesSet[name])
	}
}

func (b *BlockingKVStore[T, K]) Set(name T, value K) {
	if b == nil {
		return
	}

	b.mtx.Lock()
	defer b.mtx.Unlock()
	b.values[name] = value

	if _, exists := b.valuesSet[name]; !exists {
		b.valuesSet[name] = make(chan struct{})
	}
	select {
	case <-b.valuesSet[name]:
	default:
		close(b.valuesSet[name])
	}
}

func (b *BlockingKVStore[T, K]) Get(name T) (K, bool) {
	if b == nil {
		return *new(K), false
	}

	b.mtx.Lock()

	if _, isTracking := b.valuesSet[name]; !isTracking {
		b.valuesSet[name] = make(chan struct{})
	}

	b.mtx.Unlock()

	<-b.valuesSet[name]

	b.mtx.Lock()
	defer b.mtx.Unlock()
	if _, isUnset := b.unsetValues[name]; isUnset {
		return *new(K), false
	}
	return b.values[name], true
}
