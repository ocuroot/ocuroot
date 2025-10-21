package refstore

import (
	"encoding/json"
	"testing"
)

func TestInMemoryBackend(t *testing.T) {
	doTestBackendSetGet(t, func() DocumentBackend {
		return &inMemoryBackend{
			storage: make(map[string]json.RawMessage),
		}
	})
	doTestBackendMatch(t, func() DocumentBackend {
		return &inMemoryBackend{
			storage: make(map[string]json.RawMessage),
		}
	})
	doTestBackendInfo(t, func() DocumentBackend {
		return &inMemoryBackend{
			storage: make(map[string]json.RawMessage),
		}
	})
}
