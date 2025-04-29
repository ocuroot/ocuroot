package models

import (
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/ocuroot/ocuroot/sdk"
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
		expected SummarizedStatus
	}{
		{
			name: "empty release is complete",
			summary: ReleaseSummary{
				ID:     ReleaseID(newID()),
				Phases: []PhaseSummary{},
			},
			expected: SummarizedStatusComplete,
		},
		{
			name: "release with all complete phases is complete",
			summary: ReleaseSummary{
				ID: ReleaseID(newID()),
				Phases: []PhaseSummary{
					createPhaseSummary(SummarizedStatusComplete),
					createPhaseSummary(SummarizedStatusComplete),
				},
			},
			expected: SummarizedStatusComplete,
		},
		{
			name: "release with any pending phase is pending",
			summary: ReleaseSummary{
				ID: ReleaseID(newID()),
				Phases: []PhaseSummary{
					createPhaseSummary(SummarizedStatusComplete),
					createPhaseSummary(SummarizedStatusPending),
					createPhaseSummary(SummarizedStatusComplete),
				},
			},
			expected: SummarizedStatusPending,
		},
		{
			name: "release with any running phase is running",
			summary: ReleaseSummary{
				ID: ReleaseID(newID()),
				Phases: []PhaseSummary{
					createPhaseSummary(SummarizedStatusComplete),
					createPhaseSummary(SummarizedStatusRunning),
					createPhaseSummary(SummarizedStatusComplete),
				},
			},
			expected: SummarizedStatusRunning,
		},
		{
			name: "release with any failed phase is failed",
			summary: ReleaseSummary{
				ID: ReleaseID(newID()),
				Phases: []PhaseSummary{
					createPhaseSummary(SummarizedStatusComplete),
					createPhaseSummary(SummarizedStatusFailed),
					createPhaseSummary(SummarizedStatusComplete),
				},
			},
			expected: SummarizedStatusFailed,
		},
		{
			name: "release with any cancelled phase is cancelled",
			summary: ReleaseSummary{
				ID: ReleaseID(newID()),
				Phases: []PhaseSummary{
					createPhaseSummary(SummarizedStatusComplete),
					createPhaseSummary(SummarizedStatusCancelled),
					createPhaseSummary(SummarizedStatusComplete),
				},
			},
			expected: SummarizedStatusCancelled,
		},
		{
			name: "release returns first non-complete phase's status",
			summary: ReleaseSummary{
				ID: ReleaseID(newID()),
				Phases: []PhaseSummary{
					createPhaseSummary(SummarizedStatusComplete),
					createPhaseSummary(SummarizedStatusPending),
					createPhaseSummary(SummarizedStatusRunning),
					createPhaseSummary(SummarizedStatusCancelled),
					createPhaseSummary(SummarizedStatusFailed),
				},
			},
			expected: SummarizedStatusPending,
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
		expected SummarizedStatus
	}{
		{
			name: "empty phase is complete",
			phase: PhaseSummary{
				ID:   PhaseID(newID()),
				Name: "Empty Phase",
				Work: []WorkSummary{},
			},
			expected: SummarizedStatusComplete,
		},
		{
			name: "phase with all complete chains is complete",
			phase: PhaseSummary{
				ID:   PhaseID(newID()),
				Name: "Complete Phase",
				Work: []WorkSummary{
					createWorkSummary(SummarizedStatusComplete),
					createWorkSummary(SummarizedStatusComplete),
				},
			},
			expected: SummarizedStatusComplete,
		},
		{
			name: "phase with any pending chain is pending",
			phase: PhaseSummary{
				ID:   PhaseID(newID()),
				Name: "Pending Phase",
				Work: []WorkSummary{
					createWorkSummary(SummarizedStatusComplete),
					createWorkSummary(SummarizedStatusPending),
				},
			},
			expected: SummarizedStatusPending,
		},
		{
			name: "phase with any running chain is running",
			phase: PhaseSummary{
				ID:   PhaseID(newID()),
				Name: "Running Phase",
				Work: []WorkSummary{
					createWorkSummary(SummarizedStatusComplete),
					createWorkSummary(SummarizedStatusRunning),
				},
			},
			expected: SummarizedStatusRunning,
		},
		{
			name: "phase with any failed chain is failed",
			phase: PhaseSummary{
				ID:   PhaseID(newID()),
				Name: "Failed Phase",
				Work: []WorkSummary{
					createWorkSummary(SummarizedStatusComplete),
					createWorkSummary(SummarizedStatusFailed),
				},
			},
			expected: SummarizedStatusFailed,
		},
		{
			name: "phase with any cancelled chain is cancelled",
			phase: PhaseSummary{
				ID:   PhaseID(newID()),
				Name: "Cancelled Phase",
				Work: []WorkSummary{
					createWorkSummary(SummarizedStatusComplete),
					createWorkSummary(SummarizedStatusCancelled),
				},
			},
			expected: SummarizedStatusCancelled,
		},
		{
			name: "phase status priority: failed > cancelled > running > pending > complete",
			phase: PhaseSummary{
				ID:   PhaseID(newID()),
				Name: "Priority Phase",
				Work: []WorkSummary{
					createWorkSummary(SummarizedStatusComplete),
					createWorkSummary(SummarizedStatusPending),
					createWorkSummary(SummarizedStatusRunning),
					createWorkSummary(SummarizedStatusCancelled),
					createWorkSummary(SummarizedStatusFailed),
				},
			},
			expected: SummarizedStatusFailed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status := tt.phase.Status()
			assert.Equal(t, tt.expected, status)
		})
	}
}

// TestFunctionChainSummaryStatus tests the Status calculation of a FunctionChainSummary
func TestFunctionChainSummaryStatus(t *testing.T) {
	tests := []struct {
		name     string
		chain    FunctionChainSummary
		expected SummarizedStatus
	}{
		{
			name: "empty chain is pending",
			chain: FunctionChainSummary{
				ID:        FunctionChainID(newID()),
				Name:      "Empty Chain",
				Functions: []*FunctionSummary{},
			},
			expected: SummarizedStatusPending,
		},
		{
			name: "chain status is the status of the last function",
			chain: FunctionChainSummary{
				ID:   FunctionChainID(newID()),
				Name: "Status Chain",
				Functions: []*FunctionSummary{
					createFunctionSummary(SummarizedStatusComplete),
					createFunctionSummary(SummarizedStatusComplete),
					createFunctionSummary(SummarizedStatusFailed),
				},
			},
			expected: SummarizedStatusFailed,
		},
		{
			name: "chain with only one function",
			chain: FunctionChainSummary{
				ID:   FunctionChainID(newID()),
				Name: "Single Function Chain",
				Functions: []*FunctionSummary{
					createFunctionSummary(SummarizedStatusRunning),
				},
			},
			expected: SummarizedStatusRunning,
		},
		{
			name: "chain with multiple functions in various states",
			chain: FunctionChainSummary{
				ID:   FunctionChainID(newID()),
				Name: "Multi-Function Chain",
				Functions: []*FunctionSummary{
					createFunctionSummary(SummarizedStatusComplete),
					createFunctionSummary(SummarizedStatusFailed),
					createFunctionSummary(SummarizedStatusPending),
				},
			},
			expected: SummarizedStatusPending,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status := tt.chain.Status()
			assert.Equal(t, tt.expected, status)
		})
	}
}

// TestStatusCountMap tests the StatusCountMap functionality
func TestStatusCountMap(t *testing.T) {
	t.Run("new status count map", func(t *testing.T) {
		counts := NewStatusCountMap()
		assert.Equal(t, 0, counts[SummarizedStatusPending])
		assert.Equal(t, 0, counts[SummarizedStatusRunning])
		assert.Equal(t, 0, counts[SummarizedStatusComplete])
		assert.Equal(t, 0, counts[SummarizedStatusFailed])
		assert.Equal(t, 0, counts[SummarizedStatusCancelled])
		assert.Equal(t, 0, counts[SummarizedStatusReady])
	})

	t.Run("total count", func(t *testing.T) {
		counts := NewStatusCountMap()
		counts[SummarizedStatusPending] = 2
		counts[SummarizedStatusRunning] = 3
		counts[SummarizedStatusComplete] = 5
		counts[SummarizedStatusFailed] = 1
		assert.Equal(t, 11, counts.Total())
	})

	t.Run("completion fraction", func(t *testing.T) {
		counts := NewStatusCountMap()
		counts[SummarizedStatusPending] = 2
		counts[SummarizedStatusRunning] = 2
		counts[SummarizedStatusComplete] = 6
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
	allCompleteRelease := createRelease([][]SummarizedStatus{
		{SummarizedStatusComplete, SummarizedStatusComplete},
		{SummarizedStatusComplete, SummarizedStatusComplete},
	})
	assert.Equal(t, SummarizedStatusComplete, allCompleteRelease.Status())

	// Case 2: Release with a failed function in one chain
	failedFunctionRelease := createRelease([][]SummarizedStatus{
		{SummarizedStatusComplete, SummarizedStatusComplete},
		{SummarizedStatusComplete, SummarizedStatusFailed},
	})
	assert.Equal(t, SummarizedStatusFailed, failedFunctionRelease.Status())

	// Case 3: Release with mixed statuses
	mixedStatusRelease := createRelease([][]SummarizedStatus{
		{SummarizedStatusComplete, SummarizedStatusComplete},
		{SummarizedStatusRunning, SummarizedStatusPending},
		{SummarizedStatusComplete, SummarizedStatusComplete},
	})
	assert.Equal(t, SummarizedStatusRunning, mixedStatusRelease.Status())

	// Case 4: Release with pending functions
	pendingRelease := createRelease([][]SummarizedStatus{
		{SummarizedStatusComplete, SummarizedStatusComplete},
		{SummarizedStatusComplete, SummarizedStatusPending},
	})
	assert.Equal(t, SummarizedStatusPending, pendingRelease.Status())
}

// Helper function to create a PhaseSummary with a specific status
func createPhaseSummary(status SummarizedStatus) PhaseSummary {
	return PhaseSummary{
		ID:   PhaseID(newID()),
		Name: "Test Phase",
		Work: []WorkSummary{
			createWorkSummary(status),
		},
	}
}

// Helper function to create a WorkSummary with a specific status
func createWorkSummary(status SummarizedStatus) WorkSummary {
	return WorkSummary{
		Environment: &EnvironmentSummary{
			ID:   EnvironmentID(newID()),
			Name: "Test Environment",
		},
		Chain: &FunctionChainSummary{
			ID:   FunctionChainID(newID()),
			Name: "Test Chain",
			Functions: []*FunctionSummary{
				createFunctionSummary(status),
			},
		},
	}
}

// Helper function to create a FunctionSummary with a specific status
func createFunctionSummary(status SummarizedStatus) *FunctionSummary {
	return &FunctionSummary{
		ID: FunctionID(newID()),
		Fn: sdk.FunctionDef{
			Name: "Test Function",
		},
		Status: status,
		Inputs: map[string]sdk.InputDescriptor{
			"input": {
				Default: "test-input",
			},
		},
		Outputs: map[string]any{
			"output": "test-output",
		},
	}
}

// Helper function to create a complete release with phases and chains
func createRelease(phaseStatuses [][]SummarizedStatus) *ReleaseSummary {
	phases := make([]PhaseSummary, 0, len(phaseStatuses))

	// Create a phase for each status array
	for i, chainStatuses := range phaseStatuses {
		work := make([]WorkSummary, 0, len(chainStatuses))

		// Create chains with the specified statuses
		for j, status := range chainStatuses {
			functions := make([]*FunctionSummary, 0, 1)

			// Add a function with the specified status
			functions = append(functions, createFunctionSummary(status))

			// Create the chain
			chain := &FunctionChainSummary{
				ID:        FunctionChainID(newID()),
				Name:      fmt.Sprintf("Chain %d-%d", i, j),
				Functions: functions,
			}

			// Add the chain to the work
			work = append(work, WorkSummary{
				Environment: &EnvironmentSummary{
					ID:   EnvironmentID(newID()),
					Name: fmt.Sprintf("Environment %d-%d", i, j),
				},
				Chain: chain,
			})
		}

		// Create the phase
		phases = append(phases, PhaseSummary{
			ID:   PhaseID(newID()),
			Name: fmt.Sprintf("Phase %d", i),
			Work: work,
		})
	}

	// Create the release with all phases
	return &ReleaseSummary{
		ID:     ReleaseID(newID()),
		Phases: phases,
	}
}
