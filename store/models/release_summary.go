package models

import (
	"fmt"

	"github.com/ocuroot/ocuroot/refs"
	"github.com/ocuroot/ocuroot/sdk"
)

// SummarizedStatus represents a higher-level status that includes Ready state
type SummarizedStatus string

// SummarizedStatus constants
const (
	SummarizedStatusPending   SummarizedStatus = "pending"
	SummarizedStatusRunning   SummarizedStatus = "running"
	SummarizedStatusComplete  SummarizedStatus = "complete"
	SummarizedStatusFailed    SummarizedStatus = "failed"
	SummarizedStatusCancelled SummarizedStatus = "cancelled"
	SummarizedStatusReady     SummarizedStatus = "ready"
)

type ReleaseSummary struct {
	ID     ReleaseID      `json:"release_id"`
	Commit string         `json:"commit"`
	Phases []PhaseSummary `json:"phases"`
	Tags   map[string]any `json:"tags"`
}

func (rs *ReleaseSummary) Status() SummarizedStatus {
	for _, phase := range rs.Phases {
		ps := phase.Status()
		if ps != SummarizedStatusComplete {
			return ps
		}
	}
	return SummarizedStatusComplete
}

func (rs *ReleaseSummary) GetOutputForEnvironment(environmentName, outputName string) (*any, error) {
	for _, phase := range rs.Phases {
		for _, work := range phase.Work {
			if work.Environment != nil && work.Environment.Name == environmentName {
				if output, exists := work.Chain.Outputs()[outputName]; exists {
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
			if work.Chain.Name == workName {
				if output, exists := work.Chain.Outputs()[outputName]; exists {
					return &output, nil
				}
				return nil, fmt.Errorf("output %s not found for work %s", outputName, workName)
			}
		}
	}

	return nil, fmt.Errorf("work %s not found", workName)
}

func (rs *ReleaseSummary) FuncChainByID(id FunctionChainID) *FunctionChainSummary {
	for _, phase := range rs.Phases {
		for _, work := range phase.Work {
			if work.Chain != nil && work.Chain.ID == id {
				return work.Chain
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
	ID        FunctionChainID    `json:"id"`
	Name      string             `json:"name"`
	Functions []*FunctionSummary `json:"functions"`
	Graph     []HandoffEdge      `json:"graph,omitempty"`
}

func (fcs FunctionChainSummary) Status() SummarizedStatus {
	if len(fcs.Functions) == 0 {
		return SummarizedStatusPending
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

type FunctionSummary struct {
	ID           FunctionID                     `json:"id"`
	Fn           sdk.FunctionDef                `json:"fn"`
	Status       SummarizedStatus               `json:"status"`
	Dependencies []refs.Ref                     `json:"dependencies,omitempty"`
	Inputs       map[string]sdk.InputDescriptor `json:"inputs"`
	Outputs      map[string]any                 `json:"outputs,omitempty"`
}

type EnvironmentSummary struct {
	ID   EnvironmentID `json:"id"`
	Name string        `json:"name"`
}

// StatusCountMap tracks the count of items in each status state
type StatusCountMap map[SummarizedStatus]int

// NewStatusCountMap creates a new StatusCountMap with all statuses initialized to zero
func NewStatusCountMap() StatusCountMap {
	return StatusCountMap{
		SummarizedStatusPending:   0,
		SummarizedStatusReady:     0,
		SummarizedStatusRunning:   0,
		SummarizedStatusComplete:  0,
		SummarizedStatusFailed:    0,
		SummarizedStatusCancelled: 0,
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
	return float64(m[SummarizedStatusComplete]) / float64(total)
}

type PhaseSummary struct {
	ID   PhaseID       `json:"id"`
	Name string        `json:"name"`
	Work []WorkSummary `json:"work"`
}

func (ps *PhaseSummary) Status() SummarizedStatus {
	counts := ps.StatusCounts()
	if counts[SummarizedStatusFailed] > 0 {
		return SummarizedStatusFailed
	}
	if counts[SummarizedStatusCancelled] > 0 {
		return SummarizedStatusCancelled
	}
	if counts[SummarizedStatusRunning] > 0 {
		return SummarizedStatusRunning
	}
	if counts[SummarizedStatusPending] > 0 {
		return SummarizedStatusPending
	}
	return SummarizedStatusComplete
}

// StatusCounts returns a StatusCountMap showing how many function chains are in each status.
func (ps *PhaseSummary) StatusCounts() StatusCountMap {
	counts := NewStatusCountMap()

	// Count all chains by environment
	for _, work := range ps.Work {
		if work.Chain != nil {
			counts[work.Chain.Status()]++
		}
	}

	return counts
}

type WorkSummary struct {
	Environment *EnvironmentSummary   `json:"environment"`
	Chain       *FunctionChainSummary `json:"chain"`
}

func (ws *WorkSummary) AddSubPath(ref refs.Ref) refs.Ref {
	if ws.Chain == nil {
		return ref
	}

	if ws.Environment != nil {
		return ref.SetSubPathType(refs.SubPathTypeDeploy).SetSubPath(ws.Environment.Name)
	}
	return ref.SetSubPathType(refs.SubPathTypeCall).SetSubPath(ws.Chain.Name)
}
