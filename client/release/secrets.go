package release

import "github.com/ocuroot/ocuroot/client/local"

var _ local.KVStore[string, string] = (*SecretStore)(nil)

type SecretStore struct {
}

func NewSecretStore() *SecretStore {
	return &SecretStore{}
}

func (s *SecretStore) Waiting() []string {
	return nil
}

func (s *SecretStore) Unset(name string) {
	// TODO
}

func (s *SecretStore) Set(name string, value string) {
	// TODO
}

func (s *SecretStore) Get(name string) (string, bool) {
	return "", false
}
