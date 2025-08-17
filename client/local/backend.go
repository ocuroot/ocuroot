package local

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/charmbracelet/log"
	"github.com/ocuroot/ocuroot/refs"
	"github.com/ocuroot/ocuroot/sdk"
	"go.opentelemetry.io/otel/attribute"
	"go.starlark.net/starlark"
)

func BackendForRepo() (sdk.Backend, *BackendOutputs) {
	be := &BackendOutputs{}
	sb := &SecretsBackend{}
	backend := sdk.Backend{
		Secrets: sb,
		Http:    &HTTPBackend{},
		Repo:    &RepoBackend{Outputs: be},
		Store:   &StoreBackend{Outputs: be},
		Host:    &HostBackend{},
		Debug:   &DebugBackend{},
		Print:   &PrintBackend{Secrets: sb},
	}
	return backend, be
}

type BackendOutputs struct {
	RepoAlias    string
	RepoTrigger  *starlark.Function
	Environments []sdk.Environment
	Store        *sdk.Store
}

func NewBackend(parentRef refs.Ref) (sdk.Backend, *BackendOutputs) {
	wd, err := os.Getwd()
	if err != nil {
		log.Error("failed to get working directory", "error", err)
		wd = ""
	}
	packageDir := filepath.Join(wd, filepath.Dir(parentRef.Filename))

	be := &BackendOutputs{}

	sb := &SecretsBackend{}

	return sdk.Backend{
		AllowPackageRegistration: true,
		Http:                     &HTTPBackend{},
		Secrets:                  sb,
		Host:                     &HostBackend{WorkingDirectory: packageDir},
		Store:                    &StoreBackend{Outputs: be},
		Debug:                    &DebugBackend{},
		Refs:                     sdk.NewRefBackend(parentRef),
		Environments:             &EnvironmentBackend{Outputs: be},
		Repo:                     &RepoBackend{Outputs: be},
		Print:                    &PrintBackend{Secrets: sb},
	}, be
}

type PrintBackend struct {
	Secrets *SecretsBackend
}

// Print implements sdk.PrintBackend.
func (p *PrintBackend) Print(thread *starlark.Thread, msg string, next func(thread *starlark.Thread, msg string)) {
	for _, secret := range p.Secrets.Values {
		msg = strings.ReplaceAll(msg, secret, "<secret>")
	}
	next(thread, msg)
}

type RepoBackend struct {
	Outputs *BackendOutputs
}

// Alias implements sdk.RepoBackend.
func (r *RepoBackend) Alias(ctx context.Context, alias string) error {
	if alias == "" {
		return fmt.Errorf("alias cannot be empty")
	}
	r.Outputs.RepoAlias = alias
	return nil
}

func (r *RepoBackend) Trigger(ctx context.Context, fn *starlark.Function) {
	r.Outputs.RepoTrigger = fn
}

type EnvironmentBackend struct {
	ExistingEnvironments []sdk.Environment
	Outputs              *BackendOutputs
}

func (e *EnvironmentBackend) All(ctx context.Context) ([]sdk.Environment, error) {
	return e.ExistingEnvironments, nil
}

// Register implements sdk.EnvironmentBackend.
func (e *EnvironmentBackend) Register(ctx context.Context, env sdk.Environment) error {
	e.Outputs.Environments = append(e.Outputs.Environments, env)
	return nil
}

type HTTPBackend struct {
}

// Get implements sdk.HTTPBackend.
func (h *HTTPBackend) Get(ctx context.Context, req sdk.HTTPGetRequest) (sdk.HTTPResponse, error) {
	_, span := tracer.Start(ctx, "http get")
	defer span.End()

	span.SetAttributes(
		attribute.String("url", req.URL),
	)

	client := &http.Client{}
	httpReq, err := http.NewRequest("GET", req.URL, nil)
	if err != nil {
		return sdk.HTTPResponse{}, err
	}
	for k, a := range req.Headers {
		for _, v := range a {
			httpReq.Header.Set(k, v)
		}
	}
	resp, err := client.Do(httpReq)
	if err != nil {
		return sdk.HTTPResponse{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return sdk.HTTPResponse{}, fmt.Errorf("non-2xx response code from %s: %d", req.URL, resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return sdk.HTTPResponse{}, err
	}
	return sdk.HTTPResponse{
		Body:       string(body),
		Headers:    resp.Header,
		StatusCode: resp.StatusCode,
		StatusText: resp.Status,
	}, nil
}

// Post implements sdk.HTTPBackend.
func (h *HTTPBackend) Post(ctx context.Context, req sdk.HTTPPostRequest) (sdk.HTTPResponse, error) {
	_, span := tracer.Start(ctx, "http post")
	defer span.End()

	span.SetAttributes(
		attribute.String("url", req.URL),
	)

	client := &http.Client{}
	httpReq, err := http.NewRequest("POST", req.URL, strings.NewReader(req.Body))
	if err != nil {
		return sdk.HTTPResponse{}, err
	}
	for k, a := range req.Headers {
		for _, v := range a {
			httpReq.Header.Set(k, v)
		}
	}
	resp, err := client.Do(httpReq)
	if err != nil {
		return sdk.HTTPResponse{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return sdk.HTTPResponse{}, fmt.Errorf("non-2xx response code from %s: %d", req.URL, resp.StatusCode)
	}
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return sdk.HTTPResponse{}, err
	}
	return sdk.HTTPResponse{
		Body:       string(respBody),
		Headers:    resp.Header,
		StatusCode: resp.StatusCode,
		StatusText: resp.Status,
	}, nil
}

type SecretsBackend struct {
	Values []string
}

// Register implements sdk.SecretsBackend.
func (s *SecretsBackend) Register(value string) {
	s.Values = append(s.Values, value)
}

var _ sdk.HostBackend = (*HostBackend)(nil)

type HostBackend struct {
	WorkingDirectory string
}

// Shell implements sdk.HostBackend.
func (h *HostBackend) Shell(ctx context.Context, req sdk.HostShellRequest, output io.Writer) (sdk.HostShellResponse, error) {
	_, span := tracer.Start(ctx, "shell")
	defer span.End()

	span.SetAttributes(
		attribute.String("cmd", req.Cmd),
		attribute.String("dir", req.Dir),
		attribute.String("shell", req.Shell),
	)

	shell := "sh"
	if req.Shell != "" {
		shell = req.Shell
	}
	cmd := exec.Command(shell, "-c", req.Cmd)
	if req.Dir != "" {
		cmd.Dir = req.Dir
	}
	// Make sure we retain the existing environment
	if len(req.Env) > 0 {
		cmd.Env = os.Environ()
	}
	for k, v := range req.Env {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
	}

	if h.WorkingDirectory != "" {
		cmd.Dir = h.WorkingDirectory
	}

	var stdout, stderr, combined bytes.Buffer
	cmd.Stdout = io.MultiWriter(&stdout, &combined, output)
	cmd.Stderr = io.MultiWriter(&stderr, &combined, output)

	var outErr error
	if err := cmd.Run(); err != nil && !req.ContinueOnError {
		outErr = fmt.Errorf("%v: %w", req.Cmd, err)
	}

	return sdk.HostShellResponse{
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		ExitCode: cmd.ProcessState.ExitCode(),
	}, outErr

}

// OS implements sdk.HostBackend.
func (h *HostBackend) OS() string {
	return runtime.GOOS
}

// Arch implements sdk.HostBackend.
func (h *HostBackend) Arch() string {
	return runtime.GOARCH
}

// Env implements sdk.HostBackend.
func (h *HostBackend) Env() map[string]string {
	env := make(map[string]string)
	for _, envVar := range os.Environ() {
		i := strings.Index(envVar, "=")
		if i < 0 {
			continue
		}
		key := envVar[0:i]
		value := envVar[i+1:]
		env[key] = value
	}
	return env
}

// IsDir implements sdk.HostBackend.
func (h *HostBackend) IsDir(ctx context.Context, path string) (bool, error) {
	info, err := os.Stat(filepath.Join(h.WorkingDirectory, path))
	if err != nil {
		return false, err
	}
	return info.IsDir(), nil
}

// ReadDir implements sdk.HostBackend.
func (h *HostBackend) ReadDir(ctx context.Context, path string) ([]string, error) {
	isDir, err := h.IsDir(ctx, path)
	if err != nil {
		return nil, err
	}
	if !isDir {
		return nil, fmt.Errorf("path %s is not a directory", path)
	}
	entries, err := os.ReadDir(filepath.Join(h.WorkingDirectory, path))
	if err != nil {
		return nil, err
	}
	var names []string
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	return names, nil
}

// ReadFile implements sdk.HostBackend.
func (h *HostBackend) ReadFile(ctx context.Context, path string) (string, error) {
	data, err := os.ReadFile(filepath.Join(h.WorkingDirectory, path))
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// WorkingDir implements sdk.HostBackend.
func (h *HostBackend) WorkingDir() string {
	return h.WorkingDirectory
}

// WriteFile implements sdk.HostBackend.
func (h *HostBackend) WriteFile(ctx context.Context, req sdk.WriteFileRequest) error {
	fp := filepath.Join(h.WorkingDirectory, req.Path)
	if err := os.MkdirAll(filepath.Dir(fp), 0755); err != nil {
		return err
	}

	return os.WriteFile(fp, []byte(req.Content), 0644)
}

type StoreBackend struct {
	Outputs *BackendOutputs
}

func (s *StoreBackend) Set(store sdk.Store) error {
	if s.Outputs.Store != nil {
		return fmt.Errorf("store already set")
	}
	s.Outputs.Store = &store
	return nil
}

type DebugBackend struct {
}

func (d *DebugBackend) Break(frame sdk.DebugFrame) {
	frameJSON, err := json.MarshalIndent(frame, "", "  ")
	if err != nil {
		log.Error("failed to marshal frame", "error", err)
		frameJSON = []byte("<failed to marshal>")
	}
	log.Info("break called", "frame", string(frameJSON))
}
