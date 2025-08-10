package pipeline

import (
	"fmt"

	"github.com/ocuroot/ocuroot/refs"
	"github.com/ocuroot/ocuroot/store/models"
)

type ReleaseSummary struct {
	ID     models.ReleaseID `json:"release_id"`
	Commit string           `json:"commit"`
	Phases []PhaseSummary   `json:"phases"`
	Tags   map[string]any   `json:"tags"`
}

func (rs *ReleaseSummary) Status() models.Status {
	for _, phase := range rs.Phases {
		if ps := phase.Status(); ps != models.StatusComplete {
			return ps
		}
	}
	return models.StatusComplete
}

func (rs *ReleaseSummary) GetOutputForEnvironment(environmentName, outputName string) (*any, error) {
	for _, phase := range rs.Phases {
		for _, work := range phase.Work {
			if work.Environment != nil && work.Environment.Name == environmentName {
				if len(work.Chains) == 0 {
					continue
				}
				chain := work.Chains[len(work.Chains)-1]
				if output, exists := chain.Outputs()[outputName]; exists {
					return &output, nil
				}
				return nil, fmt.Errorf("output %s not found for environment %s", outputName, environmentName)
			}
		}
	}
	return nil, fmt.Errorf("environment %s not found", environmentName)
}

func (rs *ReleaseSummary) GetOutputForWork(workName, outputName string) (*any, error) {
	for _, phase := range rs.Phases {
		for _, work := range phase.Work {
			if len(work.Chains) == 0 {
				continue
			}
			chain := work.Chains[len(work.Chains)-1]
			if chain.Name == workName {
				if output, exists := chain.Outputs()[outputName]; exists {
					return &output, nil
				}
				return nil, fmt.Errorf("output %s not found for work %s", outputName, workName)
			}
		}
	}

	return nil, fmt.Errorf("work %s not found", workName)
}

func (rs *ReleaseSummary) FuncChainByID(id models.FunctionChainID) *FunctionChainSummary {
	for _, phase := range rs.Phases {
		for _, work := range phase.Work {
			if len(work.Chains) == 0 {
				continue
			}
			chain := work.Chains[len(work.Chains)-1]
			if chain.ID == id {
				return chain
			}
		}
	}
	return nil
}

// HandoffEdge represents a directed edge in a function chain graph
type HandoffEdge struct {
	From          string `json:"from,omitempty"`
	To            string `json:"to,omitempty"`
	Annotation    string `json:"annotation,omitempty"`
	Delay         string `json:"delay,omitempty"`
	NeedsApproval bool   `json:"needs_approval,omitempty"`
}

type FunctionChainSummary struct {
	ID        models.FunctionChainID `json:"id"`
	Name      string                 `json:"name"`
	Functions []*models.Function     `json:"functions"`
	Graph     []HandoffEdge          `json:"graph,omitempty"`
}

func (fcs FunctionChainSummary) Status() models.Status {
	if len(fcs.Functions) == 0 {
		return models.StatusPending
	}
	return fcs.Functions[len(fcs.Functions)-1].Status
}

func (fcs FunctionChainSummary) Outputs() map[string]any {
	if len(fcs.Functions) == 0 {
		return nil
	}
	lastFn := fcs.Functions[len(fcs.Functions)-1]
	return lastFn.Outputs
}

type EnvironmentSummary struct {
	ID   models.EnvironmentID `json:"id"`
	Name string               `json:"name"`
}

// StatusCountMap tracks the count of items in each status state
type StatusCountMap map[models.Status]int

// NewStatusCountMap creates a new StatusCountMap with all statuses initialized to zero
func NewStatusCountMap() StatusCountMap {
	return StatusCountMap{
		models.StatusPending:   0,
		models.StatusRunning:   0,
		models.StatusComplete:  0,
		models.StatusFailed:    0,
		models.StatusCancelled: 0,
	}
}

// Total returns the total number of items across all statuses
func (m StatusCountMap) Total() int {
	total := 0
	for _, count := range m {
		total += count
	}
	return total
}

// CompletionFraction returns the fraction of items that are complete (0.0 to 1.0)
// Returns 0 if there are no items
func (m StatusCountMap) CompletionFraction() float64 {
	total := m.Total()
	if total == 0 {
		return 0
	}
	return float64(m[models.StatusComplete]) / float64(total)
}

type PhaseSummary struct {
	ID   models.PhaseID `json:"id"`
	Name string         `json:"name"`
	Work []WorkSummary  `json:"work"`
}

func (ps *PhaseSummary) Status() models.Status {
	counts := ps.StatusCounts()
	if counts[models.StatusFailed] > 0 {
		return models.StatusFailed
	}
	if counts[models.StatusCancelled] > 0 {
		return models.StatusCancelled
	}
	if counts[models.StatusRunning] > 0 {
		return models.StatusRunning
	}
	if counts[models.StatusPending] > 0 {
		return models.StatusPending
	}
	return models.StatusComplete
}

// StatusCounts returns a StatusCountMap showing how many function chains are in each status.
func (ps *PhaseSummary) StatusCounts() StatusCountMap {
	counts := NewStatusCountMap()

	// Count latest result for all chains by environment
	for _, work := range ps.Work {
		if len(work.Chains) == 0 {
			continue
		}
		chain := work.Chains[len(work.Chains)-1]
		if chain != nil {
			counts[chain.Status()]++
		}
	}

	return counts
}

type WorkSummary struct {
	Environment *EnvironmentSummary     `json:"environment"`
	Chains      []*FunctionChainSummary `json:"chain"`
}

func (ws *WorkSummary) AddSubPath(ref refs.Ref) refs.Ref {
	if len(ws.Chains) == 0 {
		return ref
	}

	if ws.Environment != nil {
		return ref.SetSubPathType(refs.SubPathTypeDeploy).SetSubPath(ws.Environment.Name)
	}
	return ref.SetSubPathType(refs.SubPathTypeCall).SetSubPath(ws.Chains[len(ws.Chains)-1].Name)
}
