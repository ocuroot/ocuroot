package sdk

import (
	"context"
	"io"

	"github.com/ocuroot/ocuroot/refs"
)

func NewMockBackend() Backend {
	return Backend{
		Refs: NewRefBackend(refs.Ref{}),
		Environments: &mockEnvironmentBackend{
			all: []Environment{},
		},
		AllowPackageRegistration: true,
		Http:                     &mockHTTPBackend{},
		Secrets:                  &mockSecretsBackend{},
		Host:                     &mockHostBackend{},
		Store:                    &mockStoreBackend{},
		Debug:                    &mockDebugBackend{},
	}
}

type mockEnvironmentBackend struct {
	all []Environment
}

func (m *mockEnvironmentBackend) All(ctx context.Context) ([]Environment, error) {
	return m.all, nil
}

func (m *mockEnvironmentBackend) Register(ctx context.Context, env Environment) error {
	m.all = append(m.all, env)
	return nil
}

type mockHTTPBackend struct {
}

func (m *mockHTTPBackend) Get(ctx context.Context, req HTTPGetRequest) (HTTPResponse, error) {
	return HTTPResponse{}, nil
}

func (m *mockHTTPBackend) Post(ctx context.Context, req HTTPPostRequest) (HTTPResponse, error) {
	return HTTPResponse{}, nil
}

type mockSecretsBackend struct {
}

func (m *mockSecretsBackend) Register(value string) {
}

type mockHostBackend struct {
}

func (m *mockHostBackend) OS() string {
	return ""
}

func (m *mockHostBackend) Arch() string {
	return ""
}

func (m *mockHostBackend) Env() map[string]string {
	return map[string]string{}
}

func (m *mockHostBackend) Shell(ctx context.Context, req HostShellRequest, stdout io.Writer) (HostShellResponse, error) {
	return HostShellResponse{}, nil
}

func (m *mockHostBackend) WorkingDir() string {
	return ""
}

func (m *mockHostBackend) ReadFile(ctx context.Context, path string) (string, error) {
	return "", nil
}

func (m *mockHostBackend) ReadDir(ctx context.Context, path string) ([]string, error) {
	return nil, nil
}

func (m *mockHostBackend) IsDir(ctx context.Context, path string) (bool, error) {
	return false, nil
}

type WriteFileRequest struct {
	Path    string
	Content string
}

func (m *mockHostBackend) WriteFile(ctx context.Context, req WriteFileRequest) error {
	return nil
}

type mockStoreBackend struct {
}

func (m *mockStoreBackend) Set(store Store) error {
	return nil
}

type mockDebugBackend struct {
}

func (m *mockDebugBackend) Break(frame DebugFrame) {
}
