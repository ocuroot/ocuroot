package pipeline

import (
	"fmt"
	"path"

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

		for _, task := range phase.Tasks {
			ts := TaskSummary{}

			var runs []string

			var taskName string
			var function sdk.FunctionDef
			var inputs map[string]sdk.InputDescriptor
			if task.Deployment != nil {
				taskName = string(task.Deployment.Environment)
				function = task.Deployment.Up
				inputs = task.Deployment.Inputs
				ts.Environment = &EnvironmentSummary{
					ID:   models.NewID[models.EnvironmentID](),
					Name: string(task.Deployment.Environment),
				}

				// Identify any runs of this deployment
				runs = globFilter(childRefs, fmt.Sprintf("**/-/**/@*/deploy/%s/*", taskName))
			}

			if task.Task != nil {
				taskName = string(task.Task.Name)
				function = task.Task.Fn
				inputs = task.Task.Inputs

				// Identify any runs of this call
				runs = globFilter(childRefs, fmt.Sprintf("**/-/**/@*/task/%s/*", taskName))
			}

			ts.Name = taskName
			if len(runs) == 0 {
				ts.Runs = append(ts.Runs, models.Run{
					Functions: []*models.Function{
						{
							Fn:     function,
							Inputs: inputs,
						},
					},
				})
				p.Tasks = append(p.Tasks, ts)
				continue
			}

			for _, run := range runs {
				ts.Runs = append(ts.Runs, models.Run{
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
				ts.RunRefs = append(ts.RunRefs, run)
				ts.RunStatuses = append(ts.RunStatuses, status)
			}

			p.Tasks = append(p.Tasks, ts)
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
