package state

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"

	"github.com/a-h/templ"
	"github.com/ocuroot/ocuroot/lib/release"
	"github.com/ocuroot/ocuroot/refs"
)

func (s *server) handleMatch(w http.ResponseWriter, r *http.Request) {
	var err error
	query := strings.TrimPrefix(r.URL.Path, "/match/")
	query, err = url.QueryUnescape(query)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	matches, err := s.store.Match(r.Context(), query)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	partial := r.URL.Query().Get("partial") == "true"

	var content templ.Component
	switch {
	case release.GlobRepoConfig.Match(query):
		content = s.buildRepositoryTable(matches)
	case release.GlobDeployment.Match(query):
		content = s.buildDeploymentTable(r.Context(), matches)
	case release.GlobRelease.Match(query):
		content = s.buildReleaseTable(matches)
	case release.GlobTask.Match(query) || query == GlobTask:
		content = s.buildTaskTable(r.Context(), matches)
	default:
		content = Match(query, matches)
	}

	if !partial {
		content = InBody(
			MatchHeading(query),
			content,
		)
	}
	content.Render(r.Context(), w)
}

func textCell(text string) templ.Component {
	return templ.ComponentFunc(func(ctx context.Context, w io.Writer) error {
		fmt.Fprint(w, text)
		return nil
	})
}

func (s *server) buildRepositoryTable(matches []string) templ.Component {
	var tableContent []ResultTableRow
	for _, match := range matches {
		pr, err := refs.Parse(match)
		if err != nil {
			continue
		}
		tableContent = append(tableContent, ResultTableRow{
			URL: templ.URL(fmt.Sprintf("/match/%s/-/**/@r*", pr.Repo)),
			Cells: []templ.Component{
				textCell(pr.Repo),
			},
		})
	}
	return ResultTable([]string{"Repo"}, tableContent)
}

func (s *server) buildReleaseTable(matches []string) templ.Component {
	var tableContent []ResultTableRow
	for _, match := range matches {
		pr, err := refs.Parse(match)
		if err != nil {
			continue
		}
		if pr.Filename == "repo.ocu.star" {
			continue
		}
		tableContent = append(tableContent, ResultTableRow{
			URL: templ.URL(fmt.Sprintf("/ref/%s", pr.String())),
			Cells: []templ.Component{
				textCell(pr.Repo),
				textCell(pr.Filename),
				textCell(string(pr.Release)),
			},
		})
	}
	return ResultTable([]string{"Repo", "Filename", "Version"}, tableContent)
}

func (s *server) buildDeploymentTable(ctx context.Context, matches []string) templ.Component {
	var tableContent []ResultTableRow
	for _, match := range matches {
		resolved, err := s.store.ResolveLink(ctx, match)
		if err != nil {
			continue
		}
		matchParsed, err := refs.Parse(match)
		if err != nil {
			continue
		}
		resolvedParsed, err := refs.Parse(resolved)
		if err != nil {
			continue
		}
		tableContent = append(tableContent, ResultTableRow{
			URL: templ.URL(fmt.Sprintf("/ref/%s", resolvedParsed.String())),
			Cells: []templ.Component{
				textCell(resolvedParsed.Repo),
				textCell(resolvedParsed.Filename),
				textCell(matchParsed.SubPath),
				textCell(string(resolvedParsed.Release)),
			},
		})
	}
	return ResultTable([]string{"Repo", "Filename", "Environment", "Release"}, tableContent)
}

func (s *server) buildTaskTable(ctx context.Context, matches []string) templ.Component {
	var tableContent []ResultTableRow
	for _, match := range matches {
		resolved, err := s.store.ResolveLink(ctx, match)
		if err != nil {
			continue
		}
		resolvedParsed, err := refs.Parse(resolved)
		if err != nil {
			continue
		}

		var task string
		subpathSegments := strings.Split(resolvedParsed.SubPath, "/")
		switch resolvedParsed.SubPathType {
		case refs.SubPathTypeTask:
			task = fmt.Sprintf("Task '%s'", subpathSegments[0])
		case refs.SubPathTypeDeploy:
			task = fmt.Sprintf("Deploy to '%s'", subpathSegments[0])
		default:
			task = "Unknown"
		}

		var status string = path.Base(resolvedParsed.SubPath)
		taskRef := resolvedParsed.SetSubPath(path.Join(subpathSegments[0], subpathSegments[1]))

		tableContent = append(tableContent, ResultTableRow{
			URL: templ.URL(fmt.Sprintf("/ref/%s", taskRef.String())),
			Cells: []templ.Component{
				textCell(resolvedParsed.Repo),
				textCell(resolvedParsed.Filename),
				textCell(string(resolvedParsed.Release)),
				textCell(task),
				textCell(status),
			},
		})
	}
	return ResultTable([]string{"Repo", "Filename", "Release", "Task", "Status"}, tableContent)

}
