package sdk

import (
	"fmt"
)

// ValidationError represents an error found during package validation
type ValidationError struct {
	Message string
}

func (e ValidationError) Error() string {
	return e.Message
}

// Validate checks the package configuration for errors
// Currently validates:
// - Each environment is used in exactly one phase
func (p *Package) Validate() []error {
	if p == nil {
		return []error{ValidationError{Message: "Package is nil"}}
	}

	var errors []error

	// Create a map to track environment usage count across phases
	envUsage := make(map[EnvironmentName]int)

	// Count environment usage across all phases
	for _, phase := range p.Phases {
		for _, task := range phase.Tasks {
			if task.Deployment != nil {
				envUsage[task.Deployment.Environment]++
			}
		}
	}

	// Check for environments used multiple times
	for envName, count := range envUsage {
		if count > 1 {
			errors = append(errors, ValidationError{
				Message: fmt.Sprintf("Environment '%s' is used in %d phases, should be used in exactly one", envName, count),
			})
		}
	}

	// Check for duplicate names in tasks
	taskNames := make(map[string]int)
	for _, phase := range p.Phases {
		for _, task := range phase.Tasks {
			if task.Task != nil {
				taskNames[task.Task.Name]++
			}
		}
	}
	for name, count := range taskNames {
		if count > 1 {
			errors = append(errors, ValidationError{
				Message: fmt.Sprintf("Task '%s' is used in %d phases, should be used in exactly one", name, count),
			})
		}
	}

	return errors
}
