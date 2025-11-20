package refstore

import (
	"encoding/json"
	"testing"
)

func TestPrefixWrapper(t *testing.T) {
	doTestBackendSetGet(t, func() DocumentBackend {
		return &backendWithPrefix{
			backend: &inMemoryBackend{
				storage: make(map[string]json.RawMessage),
			},
		}
	})
	doTestBackendMatch(t, func() DocumentBackend {
		return &backendWithPrefix{
			backend: &inMemoryBackend{
				storage: make(map[string]json.RawMessage),
			},
		}
	})
	doTestBackendMatch(t, func() DocumentBackend {
		return &backendWithPrefix{
			backend: &inMemoryBackend{
				storage: make(map[string]json.RawMessage),
			},
			prefix: "pfx",
		}
	})
	doTestBackendInfo(t, func() DocumentBackend {
		return &backendWithPrefix{
			backend: &inMemoryBackend{
				storage: make(map[string]json.RawMessage),
			},
		}
	})
}
