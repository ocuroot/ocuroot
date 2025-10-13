package refs

import (
	"encoding/json"
	"fmt"
	"path"
	"strings"
)

type Ref struct {
	Repo     string
	Filename string
	Global   bool

	hasRelease bool
	Release    Release

	SubPathType SubPathType
	SubPath     string
	Fragment    string
}

func (r Ref) HasRelease() bool {
	return r.hasRelease
}

func (r Ref) SetRepo(repo string) Ref {
	out := r
	out.Repo = repo
	return out
}

func (r Ref) SetFilename(filename string) Ref {
	out := r
	out.Filename = filename
	return out
}

func (r Ref) SetRelease(release string) Ref {
	out := r
	out.Release = Release(release)
	out.hasRelease = true
	return out
}

func (r Ref) SetSubPathType(subPathType SubPathType) Ref {
	out := r
	out.SubPathType = subPathType
	return out
}

func (r Ref) SetSubPath(subPath string) Ref {
	out := r
	out.SubPath = subPath
	return out
}

func (r Ref) JoinSubPath(subPath ...string) Ref {
	all := append([]string{r.SubPath}, subPath...)
	out := r
	out.SubPath = path.Join(all...)
	return out
}

func (r Ref) SetFragment(fragment string) Ref {
	out := r
	out.Fragment = fragment
	return out
}

func (r Ref) IsRelative() bool {
	return r.Repo == "" || r.Repo == "." || r.Filename == "." || r.Release == "."
}

func (r Ref) IsEmpty() bool {
	if r.Repo != "" && r.Repo != "." {
		return false
	}
	if r.Filename != "" && r.Filename != "." {
		return false
	}
	if r.HasRelease() {
		return false
	}
	if r.SubPathType != "" {
		return false
	}
	if r.SubPath != "" {
		return false
	}
	if r.Fragment != "" {
		return false
	}
	return true
}

func (r Ref) Valid() error {
	if r.Global {
		if r.Filename != "" {
			return fmt.Errorf("package must be empty for global refs")
		}
	}

	if r.SubPathType != "" {
		if err := r.SubPathType.Valid(); err != nil {
			return err
		}
	}

	return nil
}

type SubPathType string

const (
	SubPathTypeNone        SubPathType = ""
	SubPathTypeDeploy      SubPathType = "deploy"
	SubPathTypeTask        SubPathType = "task"
	SubPathTypeCustom      SubPathType = "custom"
	SubPathTypeEnvironment SubPathType = "environment"
	SubPathTypeCommit      SubPathType = "commit"
	SubPathTypePush        SubPathType = "push"
	SubPathTypeOp          SubPathType = "op"
)

func (s SubPathType) Valid() error {
	switch s {
	case SubPathTypeDeploy,
		SubPathTypeTask,
		SubPathTypeCustom,
		SubPathTypeEnvironment,
		SubPathTypeCommit,
		SubPathTypePush,
		SubPathTypeOp:
		return nil
	default:
		return fmt.Errorf("invalid subpath type: %s", s)
	}
}

type Release string

func (r Release) CurrentRelease() bool {
	return r == ""
}

func (r Release) String() string {
	return string(r)
}

func (r Ref) CurrentRelease() bool {
	return r.Release.CurrentRelease()
}

// RelativeTo will return a ref based on this ref, but
// with the repo, package and release of the input ref if empty.
func (r Ref) RelativeTo(ref Ref) (Ref, error) {
	if !r.IsRelative() {
		return r, nil
	}

	if (r.Repo != "" && r.Repo != ".") || r.Global {
		return r, nil
	}

	out := r
	if r.Repo == "." || r.Repo == "" {
		out.Repo = ref.Repo
	}
	if r.Filename == "." || r.Filename == "" {
		out.Filename = ref.Filename
	} else if r.Repo == "" {
		out.Filename = path.Join(ref.Filename, r.Filename)
	}

	if !r.hasRelease && ref.hasRelease {
		out.hasRelease = true
		out.Release = ref.Release
	}

	if r.hasRelease && ref.hasRelease {
		if r.Release != "" {
			out.Release = ref.Release
		}
	}

	return out, nil
}

func (r *Ref) UnmarshalJSON(data []byte) error {
	var refStr string
	if err := json.Unmarshal(data, &refStr); err != nil {
		return fmt.Errorf("failed to unpack ref string: %w", err)
	}
	rp, err := Parse(refStr)
	if err != nil {
		return fmt.Errorf("failed to parse ref: %w", err)
	}
	*r = rp
	return nil
}

func (r Ref) MarshalJSON() ([]byte, error) {
	return json.Marshal(r.String())
}

func (r Ref) DebugString() string {
	return fmt.Sprintf("global: %t, repo: %s, package: %s, releaseOrIntent: %s, subPathType: %s, subPath: %s, fragment: %s", r.Global, r.Repo, r.Filename, r.Release, r.SubPathType, r.SubPath, r.Fragment)
}

func (r Ref) String() string {
	var segments []string

	if !r.Global {
		if r.Repo != "" {
			segments = append(segments, r.Repo)
			segments = append(segments, repoSeparator)
		}
		if r.Filename != "" {
			segments = append(segments, r.Filename)
		}
	}

	if r.hasRelease {
		segments = append(segments, fmt.Sprintf("@%s", r.Release.String()))
	}

	if r.SubPathType != "" || r.SubPath != "" {
		segments = append(segments, string(r.SubPathType), r.SubPath)
	}

	out := strings.Join(segments, "/")
	if r.Fragment != "" {
		out += "#" + r.Fragment
	}
	return out
}
