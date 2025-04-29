package models

import (
	"github.com/ocuroot/ocuroot/sdk"
)

func SDKPackageToReleaseSummary(releaseID ReleaseID, commit string, pkg *sdk.Package) *ReleaseSummary {
	summary := &ReleaseSummary{
		ID:     releaseID,
		Commit: commit,
	}

	for _, phase := range pkg.Phases {
		p := PhaseSummary{
			ID:   NewID[PhaseID](),
			Name: string(phase.Name),
		}

		for _, work := range phase.Work {
			ws := WorkSummary{}

			var chainName string
			var function sdk.FunctionDef
			var inputs map[string]sdk.InputDescriptor
			if work.Deployment != nil {
				chainName = string(work.Deployment.Environment)
				function = work.Deployment.Up
				inputs = work.Deployment.Inputs
				ws.Environment = &EnvironmentSummary{
					ID:   NewID[EnvironmentID](),
					Name: string(work.Deployment.Environment),
				}
			}

			if work.Call != nil {
				chainName = string(work.Call.Name)
				function = work.Call.Fn
				inputs = work.Call.Inputs
			}

			ws.Chain = &FunctionChainSummary{
				ID:   NewID[FunctionChainID](),
				Name: chainName,
				Functions: []*FunctionSummary{
					{
						ID:     NewID[FunctionID](),
						Fn:     function,
						Inputs: inputs,
						Status: SummarizedStatusPending,
					},
				},
				Graph: sdkGraphToHandoffGraph(pkg.Functions[function.String()].Graph),
			}
			p.Work = append(p.Work, ws)
		}

		summary.Phases = append(summary.Phases, p)
	}

	return summary
}

func sdkGraphToHandoffGraph(graph []sdk.HandoffEdge) []HandoffEdge {
	out := make([]HandoffEdge, len(graph))
	for i, edge := range graph {
		out[i] = HandoffEdge{
			From: edge.From,
			To:   edge.To,
		}
	}
	return out
}
