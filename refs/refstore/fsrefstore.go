package refstore

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	libglob "github.com/gobwas/glob"
	"github.com/ocuroot/ocuroot/refs"
)

const (
	stateVersion  = 1
	storeInfoFile = ".ocuroot-store"
	// Prefix files with @ to avoid conflicts with valid refs
	contentFile     = "@object.json"
	refMarkerFile   = "@ref.txt"
	dependenciesDir = "dependencies"
	dependantsDir   = "dependants"
	refsDir         = "refs"
)

var _ Store = (*FSStateStore)(nil)

type StoreInfo struct {
	Version int `json:"version"`
}

type StorageKind string

const (
	StorageKindRef  StorageKind = "ref"
	StorageKindLink StorageKind = "link"
)

type StorageObject struct {
	Kind     StorageKind     `json:"kind"`
	BodyType string          `json:"body_type,omitempty"`
	Links    []string        `json:"links,omitempty"`
	Body     json.RawMessage `json:"body"`
}

func NewFSRefStore(basePath string) (*FSStateStore, error) {
	f := &FSStateStore{
		BasePath: basePath,
	}

	var info StoreInfo
	infoFile := filepath.Join(f.BasePath, storeInfoFile)
	if infoBytes, err := os.ReadFile(infoFile); err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			return nil, fmt.Errorf("failed to read store info file: %v", err)
		}

		info = StoreInfo{Version: stateVersion}
		infoBytes, err = json.Marshal(info)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal store info: %v", err)
		}

		dir := filepath.Dir(infoFile)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create directory %s: %v", dir, err)
		}

		if err := os.WriteFile(infoFile, infoBytes, 0644); err != nil {
			return nil, fmt.Errorf("failed to write store info file: %v", err)
		}
	} else if err := json.Unmarshal(infoBytes, &info); err != nil {
		return nil, fmt.Errorf("failed to unmarshal store info: %v", err)
	}

	if info.Version != stateVersion {
		return nil, fmt.Errorf("incompatible store version: expected %d, got %d", stateVersion, info.Version)
	}

	return f, nil
}

type FSStateStore struct {
	BasePath string
}

func (f *FSStateStore) StartTransaction(ctx context.Context) error {
	// TODO: Implement
	return nil
}

func (f *FSStateStore) CommitTransaction(ctx context.Context, message string) error {
	// TODO: Implement
	return nil
}

// Close implements RefStore.
func (f *FSStateStore) Close() error {
	return nil
}

// Get implements RefStore.
func (f *FSStateStore) Get(ctx context.Context, ref string, v any) error {
	parsedRef, err := refs.Parse(ref)
	if err != nil {
		return fmt.Errorf("failed to parse ref: %w", err)
	}

	refWithoutFragment := parsedRef
	refWithoutFragment.Fragment = ""

	targetRef, err := f.resolveLink(refWithoutFragment.String())
	if err != nil {
		return fmt.Errorf("failed to resolve link: %w", err)
	}

	fp := f.pathToRef(targetRef)
	jsonContent, err := os.ReadFile(fp)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return ErrRefNotFound
		}
		return err
	}

	var storageObject StorageObject
	if err := json.Unmarshal(jsonContent, &storageObject); err != nil {
		return err
	}

	if storageObject.Kind != StorageKindRef {
		return fmt.Errorf("expected ref, got %s", storageObject.Kind)
	}

	if parsedRef.Fragment == "" {
		return json.Unmarshal(storageObject.Body, v)
	}

	// Walk down the map with the fragment
	var content any
	if err := json.Unmarshal(storageObject.Body, &content); err != nil {
		return err
	}

	for _, fragment := range strings.Split(parsedRef.Fragment, "/") {
		if contentMap, ok := content.(map[string]any); ok {
			if contentMap[fragment] == nil {
				return ErrRefNotFound
			}
			content = contentMap[fragment]
		} else {
			return ErrRefNotFound
		}
	}
	jsonContent, err = json.Marshal(content)
	if err != nil {
		return err
	}

	return json.Unmarshal(jsonContent, v)
}

// Set implements RefStore.
func (f *FSStateStore) Set(ctx context.Context, ref string, v any) error {
	parsedRef, err := f.resolveLink(ref)
	if err != nil {
		return fmt.Errorf("failed to resolve link: %v", err)
	}

	if parsedRef.Fragment != "" {
		return fmt.Errorf("setting by fragment not supported")
	}

	jsonBody, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}

	fp := f.pathToRef(parsedRef)

	// Create all necessary parent directories.
	if err := os.MkdirAll(filepath.Dir(fp), 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %v", filepath.Dir(fp), err)
	}

	storageObject := StorageObject{
		Kind:     StorageKindRef,
		BodyType: fmt.Sprintf("%T", v),
		Body:     jsonBody,
	}

	storageObject.Links, err = f.linksAtPath(fp)
	if err != nil {
		return fmt.Errorf("failed to get links: %v", err)
	}

	storageObjectJSON, err := json.MarshalIndent(storageObject, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal storage object: %v", err)
	}

	return os.WriteFile(fp, storageObjectJSON, 0644)
}

// Delete implements RefStore.
func (f *FSStateStore) Delete(ctx context.Context, ref string) error {
	targetRef, err := f.resolveLink(ref)
	if err != nil {
		return fmt.Errorf("failed to resolve link: %w", err)
	}

	parsedRef, err := refs.Parse(targetRef.String())
	if err != nil {
		return err
	}

	if parsedRef.Fragment != "" {
		return fmt.Errorf("delete by fragment not supported")
	}

	rpath := f.pathToRef(parsedRef)
	if err := os.Remove(rpath); err != nil {
		return err
	}
	if err := os.Remove(filepath.Dir(rpath)); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}

	return nil
}

func (f *FSStateStore) Link(ctx context.Context, ref string, target string) error {
	parsedLinkRef, err := refs.Parse(ref)
	if err != nil {
		return err
	}

	parsedTargetRef, err := refs.Parse(target)
	if err != nil {
		return err
	}

	fp := f.pathToRef(parsedLinkRef)

	if _, err := os.Stat(fp); err == nil {
		storageObjectJSON, err := os.ReadFile(fp)
		if err != nil {
			return fmt.Errorf("failed to read storage object: %v", err)
		}

		var storedRef StorageObject
		if err := json.Unmarshal(storageObjectJSON, &storedRef); err != nil {
			return fmt.Errorf("failed to unmarshal storage object: %v", err)
		}

		if storedRef.Kind != StorageKindLink {
			return fmt.Errorf("existing ref is not a link, cannot overwrite")
		}

		var oldTarget string
		if err := json.Unmarshal(storedRef.Body, &oldTarget); err != nil {
			return fmt.Errorf("failed to unmarshal storage object: %v", err)
		}

		if oldTarget == target {
			// Link already exists, no op
			return nil
		}

		oldTargetRef, err := refs.Parse(oldTarget)
		if err != nil {
			return err
		}

		if err := f.modifyRefList(oldTargetRef, ref, false); err != nil {
			return err
		}
	}

	// Create all necessary parent directories.
	if err := os.MkdirAll(filepath.Dir(fp), 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %v", filepath.Dir(fp), err)
	}

	linkJSON, err := json.Marshal(parsedTargetRef.String())
	if err != nil {
		return err
	}

	storageObject := StorageObject{
		Kind: StorageKindLink,
		Body: linkJSON,
	}

	storageObjectJSON, err := json.MarshalIndent(storageObject, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal storage object: %v", err)
	}

	err = os.WriteFile(fp, storageObjectJSON, 0644)
	if err != nil {
		return fmt.Errorf("failed to write storage object: %v", err)
	}

	var envelope StorageObject
	if err := json.Unmarshal(storageObjectJSON, &envelope); err != nil {
		return fmt.Errorf("failed to unmarshal storage object: %v", err)
	}

	if envelope.Kind != StorageKindLink {
		return fmt.Errorf("expected link, got %s", envelope.Kind)
	}

	err = f.modifyRefList(parsedTargetRef, parsedLinkRef.String(), true)
	if err != nil {
		return err
	}

	return nil
}

func (f *FSStateStore) Unlink(ctx context.Context, ref string) error {
	parsedLinkRef, err := refs.Parse(ref)
	if err != nil {
		return err
	}

	fp := f.pathToRef(parsedLinkRef)

	if _, err := os.Stat(fp); err == nil {
		err = os.Remove(fp)
		if err != nil {
			return fmt.Errorf("failed to delete file %s: %v", fp, err)
		}
	}

	return nil
}

func (f *FSStateStore) modifyRefList(ref refs.Ref, link string, add bool) error {
	fp := f.pathToRef(ref)

	var storageObject StorageObject
	storageObjectJSON, err := os.ReadFile(fp)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("failed to read storage object: %v", err)
	}

	if err := json.Unmarshal(storageObjectJSON, &storageObject); err != nil {
		return fmt.Errorf("failed to unmarshal storage object: %v", err)
	}

	var allLinks = make(map[string]struct{})
	for _, link := range storageObject.Links {
		allLinks[link] = struct{}{}
	}

	if add {
		allLinks[link] = struct{}{}
	} else {
		delete(allLinks, link)
	}

	storageObject.Links = make([]string, 0, len(allLinks))
	for link := range allLinks {
		storageObject.Links = append(storageObject.Links, link)
	}

	storageObjectJSON, err = json.MarshalIndent(storageObject, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal storage object: %v", err)
	}

	return os.WriteFile(fp, storageObjectJSON, 0644)
}

func (f *FSStateStore) GetLinks(ctx context.Context, ref string) ([]string, error) {
	parsedRef, err := refs.Parse(ref)
	if err != nil {
		return nil, err
	}

	fp := f.pathToRef(parsedRef)

	return f.linksAtPath(fp)
}

func (f *FSStateStore) linksAtPath(path string) ([]string, error) {
	var storageObject StorageObject
	storageObjectJSON, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read storage object: %v", err)
	}

	if err := json.Unmarshal(storageObjectJSON, &storageObject); err != nil {
		return nil, fmt.Errorf("failed to unmarshal storage object: %v", err)
	}

	return storageObject.Links, nil
}

func (f *FSStateStore) ResolveLink(ctx context.Context, ref string) (string, error) {
	r, err := f.resolveLink(ref)
	if err != nil {
		return "", err
	}
	return r.String(), nil
}

// Match implements RefStore.
func (f *FSStateStore) Match(ctx context.Context, glob ...string) ([]string, error) {
	refMatches, err := f.matchRefs(glob, refsDir)
	if err != nil {
		return nil, err
	}

	matchSet := make(map[string]struct{})
	for _, ref := range refMatches {
		matchSet[ref] = struct{}{}
	}

	var matches []string
	for ref := range matchSet {
		matches = append(matches, ref)
	}
	sort.Strings(matches)

	return matches, nil
}

func (f *FSStateStore) MatchOptions(ctx context.Context, options MatchOptions, glob ...string) ([]string, error) {
	rawMatches, err := f.matchRefs(glob, refsDir)
	if err != nil {
		return nil, err
	}

	var matches []string
	for _, ref := range rawMatches {
		if options.NoLinks {
			targetRef, err := f.resolveLink(ref)
			if err != nil {
				return nil, fmt.Errorf("failed to resolve link: %w", err)
			}

			if targetRef.String() != ref {
				continue
			}
		}
		matches = append(matches, ref)
	}

	return matches, nil
}

func (f *FSStateStore) matchRefs(globs []string, baseDir string) ([]string, error) {
	var compiledGlobs []libglob.Glob
	for _, glob := range globs {
		g, err := libglob.Compile(glob, '/')
		if err != nil {
			return nil, fmt.Errorf("failed to compile glob %s: %w", glob, err)
		}
		compiledGlobs = append(compiledGlobs, g)
	}

	dir := filepath.Join(f.BasePath, baseDir)
	if _, err := os.Stat(dir); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}

	var matchingRefs []string
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			return nil
		}

		if _, err := os.Stat(filepath.Join(p, contentFile)); err != nil {
			return nil
		}

		relPath, err := filepath.Rel(dir, p)
		if err != nil {
			return err
		}
		candidate := relPath
		for _, g := range compiledGlobs {
			if g.Match(candidate) {
				matchingRefs = append(matchingRefs, candidate)
				return nil
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return matchingRefs, nil
}

func (f *FSStateStore) AddDependency(ctx context.Context, ref string, dependency string) error {
	dependencyMarkerPath := filepath.Join(f.pathToDependencies(), ref, dependency, refMarkerFile)
	dependantMarkerPath := filepath.Join(f.pathToDependants(), dependency, ref, refMarkerFile)

	if err := os.MkdirAll(filepath.Dir(dependencyMarkerPath), 0755); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dependantMarkerPath), 0755); err != nil {
		return err
	}
	if err := os.WriteFile(dependencyMarkerPath, []byte(ref), 0644); err != nil {
		return err
	}
	if err := os.WriteFile(dependantMarkerPath, []byte(dependency), 0644); err != nil {
		return err
	}
	return nil
}

func (f *FSStateStore) RemoveDependency(ctx context.Context, ref string, dependency string) error {
	dependencyMarkerPath := filepath.Join(f.pathToDependencies(), ref, dependency, refMarkerFile)
	dependantMarkerPath := filepath.Join(f.pathToDependants(), dependency, ref, refMarkerFile)

	if err := os.Remove(dependencyMarkerPath); err != nil {
		return err
	}
	if err := os.Remove(dependantMarkerPath); err != nil {
		return err
	}
	return nil
}

func (f *FSStateStore) GetDependencies(ctx context.Context, ref string) ([]string, error) {
	dependencyStartPath := filepath.Join(f.pathToDependencies(), ref)
	return f.getMarkedRefsUnderPath(dependencyStartPath)
}

func (f *FSStateStore) getMarkedRefsUnderPath(p string) ([]string, error) {
	if _, err := os.Stat(p); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}

	var refs []string

	err := filepath.Walk(p, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}

		if info.Name() != refMarkerFile {
			return nil
		}

		relativePath, err := filepath.Rel(p, filepath.Dir(path))
		if err != nil {
			return err
		}

		refs = append(refs, relativePath)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return refs, nil
}

func (f *FSStateStore) GetDependants(ctx context.Context, ref string) ([]string, error) {
	dependencyStartPath := filepath.Join(f.pathToDependants(), ref)
	return f.getMarkedRefsUnderPath(dependencyStartPath)
}

func (f *FSStateStore) pathToRef(ref refs.Ref) string {
	return filepath.Join(f.BasePath, refsDir, ref.String(), contentFile)
}

func (f *FSStateStore) pathToDependencies() string {
	return filepath.Join(f.BasePath, dependenciesDir, "dependencies")
}

func (f *FSStateStore) pathToDependants() string {
	return filepath.Join(f.BasePath, dependenciesDir, "dependants")
}

func (f *FSStateStore) checkLink(ref string) (bool, string, error) {
	var storageObject StorageObject

	pr, err := refs.Parse(ref)
	if err != nil {
		return false, "", err
	}

	jsonContent, err := os.ReadFile(f.pathToRef(pr))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, "", nil
		}
		return false, "", err
	}

	if err := json.Unmarshal(jsonContent, &storageObject); err != nil {
		return false, "", err
	}

	var outRef string
	if storageObject.Kind != StorageKindLink {
		return false, "", nil
	}

	if err := json.Unmarshal(storageObject.Body, &outRef); err != nil {
		return false, "", err
	}

	return true, outRef, nil
}

func (f *FSStateStore) resolveLink(ref string) (refs.Ref, error) {
	var (
		foundLink    string
		resolvedLink string
	)
	parsedRef, err := refs.Parse(ref)
	if err != nil {
		return refs.Ref{}, err
	}

	refWithoutFragment := parsedRef
	refWithoutFragment.Fragment = ""

	for candidateRef := refWithoutFragment.String(); strings.Contains(candidateRef, "/"); candidateRef = filepath.Dir(candidateRef) {
		ok, link, err := f.checkLink(candidateRef)
		if err != nil {
			return refs.Ref{}, err
		}
		if ok {
			foundLink = candidateRef
			resolvedLink = link
			break
		}
	}

	if resolvedLink == "" {
		return parsedRef, nil
	}

	resolvedRef := strings.Replace(ref, foundLink, resolvedLink, 1)

	out, err := refs.Parse(resolvedRef)
	if err != nil {
		return refs.Ref{}, fmt.Errorf("%v: %w", resolvedRef, err)
	}

	return out, nil
}
