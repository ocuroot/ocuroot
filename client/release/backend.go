package release

import (
	"context"
	"os"
	"path/filepath"

	"github.com/charmbracelet/log"
	"github.com/ocuroot/ocuroot/client/local"
	"github.com/ocuroot/ocuroot/refs/refstore"
	"github.com/ocuroot/ocuroot/sdk"
)

func NewBackend(tc TrackerConfig) (sdk.Backend, *local.BackendOutputs) {
	wd, err := os.Getwd()
	if err != nil {
		log.Error("failed to get working directory", "error", err)
		wd = ""
	}
	packageDir := filepath.Join(wd, filepath.Dir(tc.Ref.Filename))

	be := &local.BackendOutputs{}
	return sdk.Backend{
		AllowPackageRegistration: true,
		Http:                     &local.HTTPBackend{},
		Secrets:                  &local.SecretsBackend{SecretStore: NewSecretStore()},
		Host:                     &local.HostBackend{WorkingDirectory: packageDir},
		Store:                    &local.StoreBackend{Outputs: be},
		Debug:                    &local.DebugBackend{},
		Refs:                     sdk.NewRefBackend(tc.Ref),
		Environments:             &EnvironmentBackend{Store: tc.Store, Outputs: be},
		Repo:                     &local.RepoBackend{Outputs: be},
	}, be
}

type EnvironmentBackend struct {
	Store   refstore.Store
	Outputs *local.BackendOutputs
}

func (e *EnvironmentBackend) All(ctx context.Context) ([]sdk.Environment, error) {
	environmentRefs, err := e.Store.Match(ctx, "@/environment/*")
	if err != nil {
		return nil, err
	}
	var environments []sdk.Environment
	for _, ref := range environmentRefs {
		var environment sdk.Environment
		err := e.Store.Get(ctx, ref, &environment)
		if err != nil {
			return nil, err
		}
		environments = append(environments, environment)
	}
	return environments, nil
}

// Register implements sdk.EnvironmentBackend.
func (e *EnvironmentBackend) Register(ctx context.Context, env sdk.Environment) error {
	e.Outputs.Environments = append(e.Outputs.Environments, env)
	return nil
}
