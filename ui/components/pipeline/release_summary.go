package pipeline

import (
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
		if len(work.JobStatuses) == 0 {
			continue
		}
		counts[work.JobStatuses[len(work.JobStatuses)-1]]++
	}

	return counts
}

type WorkSummary struct {
	Name        string
	Environment *EnvironmentSummary `json:"environment"`
	Jobs        []models.Work
	JobRefs     []string
	JobStatuses []models.Status
}
