package sdk

import (
	"context"
	"io"

	"github.com/ocuroot/ocuroot/refs"
	"go.starlark.net/starlark"
)

type Backend struct {
	Repo                     RepoBackend
	Refs                     RefsBackend
	Environments             EnvironmentBackend
	AllowPackageRegistration bool
	Http                     HTTPBackend
	Secrets                  SecretsBackend
	Host                     HostBackend
	Store                    StoreBackend
	Debug                    DebugBackend
}

func NewRefBackend(parentRef refs.Ref) RefsBackend {
	return &implRefsBackend{
		ParentRef: parentRef,
	}
}

type implRefsBackend struct {
	ParentRef refs.Ref
}

func (r *implRefsBackend) Absolute(ref refs.Ref) (refs.Ref, error) {
	return ref.RelativeTo(r.ParentRef)
}

type RefsBackend interface {
	Absolute(ref refs.Ref) (refs.Ref, error)
}

type RepoBackend interface {
	Alias(ctx context.Context, alias string) error
	Trigger(ctx context.Context, fn *starlark.Function)
}

type EnvironmentBackend interface {
	All(ctx context.Context) ([]Environment, error)
	Register(ctx context.Context, env Environment) error
}

type DebugFrame struct {
	Pos    string         `json:"pos"`
	Locals []DebugBinding `json:"locals"`
}

type DebugBinding struct {
	Name string `json:"name"`
	Pos  string `json:"pos"`
	Type string `json:"type"`
	Val  string `json:"val"`
}

type DebugBackend interface {
	// Break will block until the user resumes the program
	Break(frame DebugFrame)
}

type HostShellRequest struct {
	Cmd   string            `json:"cmd"`
	Env   map[string]string `json:"env"`
	Dir   string            `json:"dir"`
	Shell string            `json:"shell"`

	ContinueOnError bool `json:"continue_on_error"`
	Mute            bool `json:"mute"`
}

type HostShellResponse struct {
	CombinedOutput string `json:"combined_output"`
	Stdout         string `json:"stdout"`
	Stderr         string `json:"stderr"`
	ExitCode       int    `json:"exit_code"`
}

type HostBackend interface {
	Shell(ctx context.Context, req HostShellRequest, output io.Writer) (HostShellResponse, error)
	OS() string
	Arch() string
	Env() map[string]string
}

type HTTPResponse struct {
	Body       string              `json:"body"`
	Headers    map[string][]string `json:"headers"`
	StatusCode int                 `json:"status_code"`
	StatusText string              `json:"status_text"`
}

type HTTPGetRequest struct {
	URL     string              `json:"url"`
	Headers map[string][]string `json:"headers"`
}

type HTTPPostRequest struct {
	URL     string              `json:"url"`
	Body    string              `json:"body"`
	Headers map[string][]string `json:"headers"`
}

type HTTPBackend interface {
	Get(ctx context.Context, req HTTPGetRequest) (HTTPResponse, error)
	Post(ctx context.Context, req HTTPPostRequest) (HTTPResponse, error)
}

type SecretsBackend interface {
	Register(value string)
	Load(ctx context.Context, name string) (string, error)
}

type Store struct {
	State  StorageBackend  `json:"state"`
	Intent *StorageBackend `json:"intent,omitempty"`
}

type StorageBackend struct {
	Git *struct {
		RemoteURL string `json:"remote_url"`
		Branch    string `json:"branch"`
	} `json:"git,omitempty"`
	Fs *struct {
		Path string `json:"path"`
	} `json:"fs,omitempty"`
}

type StoreBackend interface {
	Set(store Store) error
}
