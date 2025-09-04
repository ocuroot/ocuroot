package pipeline

import (
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/ocuroot/ocuroot/sdk"
	"github.com/ocuroot/ocuroot/store/models"
	"github.com/stretchr/testify/assert"
)

// Helper function to create a unique ID
func newID() string {
	return uuid.New().String()
}

// TestReleaseSummaryStatus tests the Status calculation of a ReleaseSummary
func TestReleaseSummaryStatus(t *testing.T) {
	tests := []struct {
		name     string
		summary  ReleaseSummary
		expected models.Status
	}{
		{
			name: "empty release is complete",
			summary: ReleaseSummary{
				ID:     models.ReleaseID(newID()),
				Phases: []PhaseSummary{},
			},
			expected: models.StatusComplete,
		},
		{
			name: "release with all complete phases is complete",
			summary: ReleaseSummary{
				ID: models.ReleaseID(newID()),
				Phases: []PhaseSummary{
					createPhaseSummary(models.StatusComplete),
					createPhaseSummary(models.StatusComplete),
				},
			},
			expected: models.StatusComplete,
		},
		{
			name: "release with any pending phase is pending",
			summary: ReleaseSummary{
				ID: models.ReleaseID(newID()),
				Phases: []PhaseSummary{
					createPhaseSummary(models.StatusComplete),
					createPhaseSummary(models.StatusPending),
					createPhaseSummary(models.StatusComplete),
				},
			},
			expected: models.StatusPending,
		},
		{
			name: "release with any running phase is running",
			summary: ReleaseSummary{
				ID: models.ReleaseID(newID()),
				Phases: []PhaseSummary{
					createPhaseSummary(models.StatusComplete),
					createPhaseSummary(models.StatusRunning),
					createPhaseSummary(models.StatusComplete),
				},
			},
			expected: models.StatusRunning,
		},
		{
			name: "release with any failed phase is failed",
			summary: ReleaseSummary{
				ID: models.ReleaseID(newID()),
				Phases: []PhaseSummary{
					createPhaseSummary(models.StatusComplete),
					createPhaseSummary(models.StatusFailed),
					createPhaseSummary(models.StatusComplete),
				},
			},
			expected: models.StatusFailed,
		},
		{
			name: "release with any cancelled phase is cancelled",
			summary: ReleaseSummary{
				ID: models.ReleaseID(newID()),
				Phases: []PhaseSummary{
					createPhaseSummary(models.StatusComplete),
					createPhaseSummary(models.StatusCancelled),
					createPhaseSummary(models.StatusComplete),
				},
			},
			expected: models.StatusCancelled,
		},
		{
			name: "release returns first non-complete phase's status",
			summary: ReleaseSummary{
				ID: models.ReleaseID(newID()),
				Phases: []PhaseSummary{
					createPhaseSummary(models.StatusComplete),
					createPhaseSummary(models.StatusPending),
					createPhaseSummary(models.StatusRunning),
					createPhaseSummary(models.StatusCancelled),
					createPhaseSummary(models.StatusFailed),
				},
			},
			expected: models.StatusPending,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status := tt.summary.Status()
			assert.Equal(t, tt.expected, status)
		})
	}
}

// TestPhaseSummaryStatus tests the Status calculation of a PhaseSummary
func TestPhaseSummaryStatus(t *testing.T) {
	tests := []struct {
		name     string
		phase    PhaseSummary
		expected models.Status
	}{
		{
			name: "empty phase is complete",
			phase: PhaseSummary{
				ID:   models.PhaseID(newID()),
				Name: "Empty Phase",
				Work: []WorkSummary{},
			},
			expected: models.StatusComplete,
		},
		{
			name: "phase with all complete chains is complete",
			phase: PhaseSummary{
				ID:   models.PhaseID(newID()),
				Name: "Complete Phase",
				Work: []WorkSummary{
					createWorkSummary(models.StatusComplete),
					createWorkSummary(models.StatusComplete),
				},
			},
			expected: models.StatusComplete,
		},
		{
			name: "phase with any pending chain is pending",
			phase: PhaseSummary{
				ID:   models.PhaseID(newID()),
				Name: "Pending Phase",
				Work: []WorkSummary{
					createWorkSummary(models.StatusComplete),
					createWorkSummary(models.StatusPending),
				},
			},
			expected: models.StatusPending,
		},
		{
			name: "phase with any running chain is running",
			phase: PhaseSummary{
				ID:   models.PhaseID(newID()),
				Name: "Running Phase",
				Work: []WorkSummary{
					createWorkSummary(models.StatusComplete),
					createWorkSummary(models.StatusRunning),
				},
			},
			expected: models.StatusRunning,
		},
		{
			name: "phase with any failed chain is failed",
			phase: PhaseSummary{
				ID:   models.PhaseID(newID()),
				Name: "Failed Phase",
				Work: []WorkSummary{
					createWorkSummary(models.StatusComplete),
					createWorkSummary(models.StatusFailed),
				},
			},
			expected: models.StatusFailed,
		},
		{
			name: "phase with any cancelled chain is cancelled",
			phase: PhaseSummary{
				ID:   models.PhaseID(newID()),
				Name: "Cancelled Phase",
				Work: []WorkSummary{
					createWorkSummary(models.StatusComplete),
					createWorkSummary(models.StatusCancelled),
				},
			},
			expected: models.StatusCancelled,
		},
		{
			name: "phase status priority: failed > cancelled > running > pending > complete",
			phase: PhaseSummary{
				ID:   models.PhaseID(newID()),
				Name: "Priority Phase",
				Work: []WorkSummary{
					createWorkSummary(models.StatusComplete),
					createWorkSummary(models.StatusPending),
					createWorkSummary(models.StatusRunning),
					createWorkSummary(models.StatusCancelled),
					createWorkSummary(models.StatusFailed),
				},
			},
			expected: models.StatusFailed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status := tt.phase.Status()
			assert.Equal(t, tt.expected, status)
		})
	}
}

// TestStatusCountMap tests the StatusCountMap functionality
func TestStatusCountMap(t *testing.T) {
	t.Run("new status count map", func(t *testing.T) {
		counts := NewStatusCountMap()
		assert.Equal(t, 0, counts[models.StatusPending])
		assert.Equal(t, 0, counts[models.StatusRunning])
		assert.Equal(t, 0, counts[models.StatusComplete])
		assert.Equal(t, 0, counts[models.StatusFailed])
		assert.Equal(t, 0, counts[models.StatusCancelled])
	})

	t.Run("total count", func(t *testing.T) {
		counts := NewStatusCountMap()
		counts[models.StatusPending] = 2
		counts[models.StatusRunning] = 3
		counts[models.StatusComplete] = 5
		counts[models.StatusFailed] = 1
		assert.Equal(t, 11, counts.Total())
	})

	t.Run("completion fraction", func(t *testing.T) {
		counts := NewStatusCountMap()
		counts[models.StatusPending] = 2
		counts[models.StatusRunning] = 2
		counts[models.StatusComplete] = 6
		assert.Equal(t, 0.6, counts.CompletionFraction())
	})

	t.Run("completion fraction with zero total", func(t *testing.T) {
		counts := NewStatusCountMap()
		assert.Equal(t, 0.0, counts.CompletionFraction())
	})
}

// TestReleaseSummaryIntegration tests the entire release summary status calculation
// including phases, chains, and functions
func TestReleaseSummaryIntegration(t *testing.T) {
	// Create a release with multiple phases in different states
	// to validate the status calculation logic works across the entire structure

	// Case 1: All complete release
	allCompleteRelease := createRelease([][]models.Status{
		{models.StatusComplete, models.StatusComplete},
		{models.StatusComplete, models.StatusComplete},
	})
	assert.Equal(t, models.StatusComplete, allCompleteRelease.Status())

	// Case 2: Release with a failed function in one chain
	failedFunctionRelease := createRelease([][]models.Status{
		{models.StatusComplete, models.StatusComplete},
		{models.StatusComplete, models.StatusFailed},
	})
	assert.Equal(t, models.StatusFailed, failedFunctionRelease.Status())

	// Case 3: Release with mixed statuses
	mixedStatusRelease := createRelease([][]models.Status{
		{models.StatusComplete, models.StatusComplete},
		{models.StatusRunning, models.StatusPending},
		{models.StatusComplete, models.StatusComplete},
	})
	assert.Equal(t, models.StatusRunning, mixedStatusRelease.Status())

	// Case 4: Release with pending functions
	pendingRelease := createRelease([][]models.Status{
		{models.StatusComplete, models.StatusComplete},
		{models.StatusComplete, models.StatusPending},
	})
	assert.Equal(t, models.StatusPending, pendingRelease.Status())
}

// Helper function to create a PhaseSummary with a specific status
func createPhaseSummary(status models.Status) PhaseSummary {
	return PhaseSummary{
		ID:   models.PhaseID(newID()),
		Name: "Test Phase",
		Work: []WorkSummary{
			createWorkSummary(status),
		},
	}
}

// Helper function to create a WorkSummary with a specific status
func createWorkSummary(status models.Status) WorkSummary {
	return WorkSummary{
		Environment: &EnvironmentSummary{
			ID:   models.EnvironmentID(newID()),
			Name: "Test Environment",
		},
		Jobs: []models.Run{
			{
				Functions: []*models.Function{
					createFunctionSummary(),
				},
			},
		},
		JobStatuses: []models.Status{status},
	}
}

// Helper function to create a FunctionSummary with a specific status
func createFunctionSummary() *models.Function {
	return &models.Function{
		Fn: sdk.FunctionDef{
			Name: "Test Function",
		},
		Inputs: map[string]sdk.InputDescriptor{
			"input": {
				Default: "test-input",
			},
		},
	}
}

// Helper function to create a complete release with phases and chains
func createRelease(phaseStatuses [][]models.Status) *ReleaseSummary {
	phases := make([]PhaseSummary, 0, len(phaseStatuses))

	// Create a phase for each status array
	for i, chainStatuses := range phaseStatuses {
		work := make([]WorkSummary, 0, len(chainStatuses))

		// Create chains with the specified statuses
		for j, status := range chainStatuses {
			functions := make([]*models.Function, 0, 1)

			// Add a function with the specified status
			functions = append(functions, createFunctionSummary())

			// Create the chain
			chain := models.Run{
				Functions: functions,
			}

			// Add the chain to the work
			work = append(work, WorkSummary{
				Environment: &EnvironmentSummary{
					ID:   models.EnvironmentID(newID()),
					Name: fmt.Sprintf("Environment %d-%d", i, j),
				},
				Jobs:        []models.Run{chain},
				JobStatuses: []models.Status{status},
			})
		}

		// Create the phase
		phases = append(phases, PhaseSummary{
			ID:   models.PhaseID(newID()),
			Name: fmt.Sprintf("Phase %d", i),
			Work: work,
		})
	}

	// Create the release with all phases
	return &ReleaseSummary{
		ID:     models.ReleaseID(newID()),
		Phases: phases,
	}
}
