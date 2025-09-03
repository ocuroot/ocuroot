package pipeline

import (
	"fmt"
	"path"
	"sort"

	"github.com/charmbracelet/log"
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

			ws.Name = chainName
			if len(workRuns) == 0 {
				ws.Jobs = append(ws.Jobs, models.Work{
					Functions: []*models.Function{
						{
							Fn:     function,
							Inputs: inputs,
						},
					},
				})
				p.Work = append(p.Work, ws)
				continue
			}

			for _, run := range workRuns {
				ws.Jobs = append(ws.Jobs, models.Work{
					Functions: []*models.Function{
						{
							Fn:     function,
							Inputs: inputs,
						},
					},
				})
				statusRefs := globFilter(childRefs, fmt.Sprintf("%s/status/*", run))
				if len(statusRefs) > 0 {
					log.Error("Multiple status refs", "run", run, "refs", statusRefs)
				}
				status := models.Status(path.Base(statusRefs[0]))
				ws.JobStatuses = append(ws.JobStatuses, status)
			}

			p.Work = append(p.Work, ws)
		}

		summary.Phases = append(summary.Phases, p)
	}

	return summary
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
