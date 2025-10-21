package refstore

import (
	"context"
)

func NewFSRefStore(basePath string, tags map[string]struct{}) (Store, error) {
	return NewRefStore(context.Background(), NewFsBackend(basePath), tags)
}
