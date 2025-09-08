package release

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"

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

	sb := &local.SecretsBackend{}

	return sdk.Backend{
		AllowPackageRegistration: true,
		Http:                     &local.HTTPBackend{},
		Secrets:                  sb,
		Host:                     &local.HostBackend{WorkingDirectory: packageDir},
		Store:                    &local.StoreBackend{Outputs: be},
		Debug:                    &local.DebugBackend{},
		Refs:                     sdk.NewRefBackend(tc.Ref),
		Environments:             &EnvironmentBackend{State: tc.State, Outputs: be},
		Repo:                     &local.RepoBackend{Outputs: be},
		Print:                    &local.PrintBackend{Secrets: sb},
	}, be
}

type EnvironmentBackend struct {
	State   refstore.Store
	Outputs *local.BackendOutputs
}

func (e *EnvironmentBackend) All(ctx context.Context) ([]sdk.Environment, error) {
	environmentRefs, err := e.State.Match(ctx, "@/environment/*")
	if err != nil {
		return nil, err
	}
	var environments []sdk.Environment
	for _, ref := range environmentRefs {
		var environment sdk.Environment
		err := e.State.Get(ctx, ref, &environment)
		if err != nil {
			return nil, err
		}
		environments = append(environments, environment)
	}
	return environments, nil
}

var environmentNameRegex = regexp.MustCompile(`^[a-zA-Z0-9_\.]+$`)

func ValidateEnvironment(env sdk.Environment) error {
	if !environmentNameRegex.MatchString(string(env.Name)) {
		return fmt.Errorf("environment names may only contain letters, numbers, periods and underscores: %s", env.Name)
	}
	return nil
}

// Register implements sdk.EnvironmentBackend.
func (e *EnvironmentBackend) Register(ctx context.Context, env sdk.Environment) error {
	if err := ValidateEnvironment(env); err != nil {
		return err
	}

	e.Outputs.Environments = append(e.Outputs.Environments, env)
	return nil
}
