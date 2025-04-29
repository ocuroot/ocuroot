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

	ReleaseOrIntent ReleaseOrIntent
	SubPathType     SubPathType
	SubPath         string
	Fragment        string
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

func (r Ref) MakeIntent() Ref {
	out := r
	out.ReleaseOrIntent.Type = Intent
	return out
}

func (r Ref) MakeRelease() Ref {
	out := r
	out.ReleaseOrIntent.Type = Release
	return out
}

func (r Ref) SetVersion(version string) Ref {
	out := r
	out.ReleaseOrIntent.Value = version
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
	return r.Repo == "." || r.Filename == "." || r.ReleaseOrIntent.Value == "."
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
	SubPathTypeCall        SubPathType = "call"
	SubPathTypeCustom      SubPathType = "custom"
	SubPathTypeEnvironment SubPathType = "environment"
)

func (s SubPathType) Valid() error {
	switch s {
	case SubPathTypeDeploy, SubPathTypeCall, SubPathTypeCustom, SubPathTypeEnvironment:
		return nil
	default:
		return fmt.Errorf("invalid subpath type: %s", s)
	}
}

type ReleaseOrIntent struct {
	Type  ReleaseOrIntentType
	Value string
}

func (r ReleaseOrIntent) CurrentRelease() bool {
	return r.Type == Release && r.Value == ""
}

func (r ReleaseOrIntent) String() string {
	if r.Type == Intent {
		return "+" + r.Value
	}
	if r.Type == Release {
		return "@" + r.Value
	}
	return ""
}

type ReleaseOrIntentType int

const (
	Unknown ReleaseOrIntentType = iota // Should be the default value
	Release
	Intent
)

func (r Ref) CurrentRelease() bool {
	return r.ReleaseOrIntent.CurrentRelease()
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
	}
	if r.ReleaseOrIntent.Type == Unknown {
		out.ReleaseOrIntent = ref.ReleaseOrIntent
	}
	if r.ReleaseOrIntent.Value == "." {
		out.ReleaseOrIntent = ref.ReleaseOrIntent
	}

	if strings.Contains(r.String(), "@") {
		ref.ReleaseOrIntent = ReleaseOrIntent{
			Type:  Unknown,
			Value: "",
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
	return fmt.Sprintf("global: %t, repo: %s, package: %s, releaseOrIntent: %s, subPathType: %s, subPath: %s, fragment: %s", r.Global, r.Repo, r.Filename, r.ReleaseOrIntent, r.SubPathType, r.SubPath, r.Fragment)
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

	if r.ReleaseOrIntent.Type != Unknown {
		segments = append(segments, r.ReleaseOrIntent.String())
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

type WorkType string

const (
	WorkTypeCall   WorkType = "call"
	WorkTypeDeploy WorkType = "deploy"
)
