package pipeline

import (
	"fmt"
	"path"
	"sort"

	libglob "github.com/gobwas/glob"
	"github.com/ocuroot/ocuroot/sdk"
	"github.com/ocuroot/ocuroot/store/models"
)

func SDKPackageToReleaseSummary(
	releaseID models.ReleaseID,
	commit string,
	pkg *sdk.Package,
	childRefs ...string,
) *ReleaseSummary {
	summary := &ReleaseSummary{
		ID:     releaseID,
		Commit: commit,
	}

	for _, phase := range pkg.Phases {
		p := PhaseSummary{
			ID:   models.NewID[models.PhaseID](),
			Name: string(phase.Name),
		}

		for _, work := range phase.Work {
			ws := WorkSummary{}

			var workRuns []string

			var chainName string
			var function sdk.FunctionDef
			var inputs map[string]sdk.InputDescriptor
			if work.Deployment != nil {
				chainName = string(work.Deployment.Environment)
				function = work.Deployment.Up
				inputs = work.Deployment.Inputs
				ws.Environment = &EnvironmentSummary{
					ID:   models.NewID[models.EnvironmentID](),
					Name: string(work.Deployment.Environment),
				}

				// Identify any runs of this deployment
				workRuns = globFilter(childRefs, fmt.Sprintf("**/-/**/@*/deploy/%s/*", chainName))
			}

			if work.Call != nil {
				chainName = string(work.Call.Name)
				function = work.Call.Fn
				inputs = work.Call.Inputs

				// Identify any runs of this call
				workRuns = globFilter(childRefs, fmt.Sprintf("**/-/**/@*/call/%s/*", chainName))
			}

			id := models.NewID[models.FunctionChainID]()
			if len(workRuns) > 0 {
				id = models.FunctionChainID(workRuns[0])
			}

			ws.Chain = &FunctionChainSummary{
				ID:   id,
				Name: chainName,
				Functions: []*models.Function{
					{
						ID:     models.NewID[models.FunctionID](),
						Fn:     function,
						Inputs: inputs,
						Status: models.StatusPending,
					},
				},
				Graph: sdkGraphToHandoffGraph(pkg.Functions[function.String()].Graph),
			}

			if len(workRuns) > 0 {
				workRuns = latestFirst(workRuns)
				for _, run := range workRuns {
					functions := globFilter(childRefs, fmt.Sprintf("%s/functions/*", run))
					functions = earliestFirst(functions)
					for index, fn := range functions {
						var status models.Status = models.StatusPending
						statusRefs := globFilter(childRefs, fmt.Sprintf("%s/status/*", fn))
						if len(statusRefs) > 0 {
							status = models.Status(path.Base(statusRefs[0]))
						}
						var fn *models.Function
						if index == 0 {
							fn = ws.Chain.Functions[0]
							ws.Chain.Functions = nil
						}
						fn.Status = status
						ws.Chain.Functions = append(ws.Chain.Functions, fn)
					}
				}
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

func globFilter(refs []string, glob string) []string {
	compiledGlob := libglob.MustCompile(glob, '/')
	out := make([]string, 0)
	for _, ref := range refs {
		if compiledGlob.Match(ref) {
			out = append(out, ref)
		}
	}
	return out
}

func earliestFirst(refs []string) []string {
	sort.Slice(refs, func(i, j int) bool {
		return refs[i] < refs[j]
	})

	return refs
}

func latestFirst(refs []string) []string {
	sort.Slice(refs, func(i, j int) bool {
		return refs[i] > refs[j]
	})

	return refs
}
