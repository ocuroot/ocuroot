package refstore

import (
	"context"
	"encoding/json"
	"fmt"
	"path"
	"sort"
	"strings"

	"github.com/ocuroot/ocuroot/refs"
)

const (
	stateVersion  = 3
	storeInfoFile = ".ocuroot-store"
	// Prefix files with @ to avoid conflicts with valid refs
	contentFile     = "@object.json"
	refMarkerFile   = "@ref.txt"
	dependenciesDir = "dependencies"
	dependantsDir   = "dependants"
	refsDir         = "refs"
)

var _ Store = &RefStore{}

func NewRefStore(ctx context.Context, backend DocumentBackend, tags map[string]struct{}) (*RefStore, error) {
	info, err := backend.GetInfo(ctx)
	if err != nil {
		return nil, err
	}

	if info != nil {
		if info.Version == 2 {
			info.Version = 3
			info.Tags = tags
			if err := backend.SetInfo(ctx, info); err != nil {
				return nil, err
			}
		}

		if info.Version != stateVersion {
			return nil, fmt.Errorf("incompatible store version: expected %d, got %d", stateVersion, info.Version)
		}
	} else {
		var info StoreInfo = StoreInfo{Version: stateVersion, Tags: tags}
		err = backend.SetInfo(ctx, &info)
		if err != nil {
			return nil, err
		}
	}

	return &RefStore{
		Backend: backend,
	}, nil
}

type RefStore struct {
	Backend DocumentBackend

	InTransaction      bool
	TransactionMessage string
	TransactionActions []SetRequest
}

type TransactionAction struct {
	Ref string
	Doc *StorageObject
}

// Info implements Store.
func (r *RefStore) Info() StoreInfo {
	info, err := r.Backend.GetInfo(context.Background())
	if err != nil {
		panic(err)
	}
	return *info
}

// StartTransaction implements Store.
func (r *RefStore) StartTransaction(ctx context.Context, message string) error {
	r.TransactionMessage = message
	r.InTransaction = true
	return nil
}

// CommitTransaction implements Store.
func (r *RefStore) CommitTransaction(ctx context.Context) error {
	if !r.InTransaction {
		return nil
	}

	// Deduplicate requests so the latest one for each ref is the final state
	// This may result in deletion of refs that don't exist.
	// The backends must treat this as idempotent.
	var requestsByRef = make(map[string]SetRequest)
	for _, action := range r.TransactionActions {
		requestsByRef[action.Path] = action
	}
	var requests []SetRequest
	for _, req := range requestsByRef {
		requests = append(requests, req)
	}

	if err := r.Backend.Set(ctx, nil, r.TransactionMessage, requests); err != nil {
		return err
	}

	r.TransactionMessage = ""
	r.TransactionActions = nil
	r.InTransaction = false
	return nil
}

func (r *RefStore) handleRequest(ctx context.Context, reqs ...SetRequest) error {
	if r.InTransaction {
		r.TransactionActions = append(r.TransactionActions, reqs...)
		return nil
	}

	return r.Backend.Set(ctx, nil, "", reqs)
}

// Close implements Store.
func (r *RefStore) Close() error {
	// TODO: Implement for all backends
	return nil
}

func refContentPath(ref string) string {
	return path.Join(refsDir, ref, contentFile)
}

// Delete implements Store.
func (r *RefStore) Delete(ctx context.Context, ref string) error {
	return r.handleRequest(ctx, SetRequest{
		Path: refContentPath(ref),
		Doc:  nil,
	})
}

// Get implements Store.
func (r *RefStore) Get(ctx context.Context, ref string, v any) error {
	parsedRef, err := refs.Parse(ref)
	if err != nil {
		return fmt.Errorf("failed to parse ref: %w", err)
	}

	refWithoutFragment := parsedRef
	refWithoutFragment.Fragment = ""

	resolved, err := r.ResolveLink(ctx, refWithoutFragment.String())
	if err != nil {
		return err
	}

	storageObject, err := r.getRef(ctx, resolved)
	if err != nil {
		return err
	}
	if storageObject == nil {
		return ErrRefNotFound
	}

	if parsedRef.Fragment == "" {
		return json.Unmarshal(storageObject.Body, v)
	}

	var content any
	if err := json.Unmarshal(storageObject.Body, &content); err != nil {
		return fmt.Errorf("unmarshal body: %w", err)
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
	jsonContent, err := json.Marshal(content)
	if err != nil {
		return fmt.Errorf("marshal fragment: %w", err)
	}

	err = json.Unmarshal(jsonContent, v)
	if err != nil {
		return fmt.Errorf("unmarshal json fragment: %w", err)
	}
	return nil
}

func (r *RefStore) getRef(ctx context.Context, ref string) (*StorageObject, error) {
	rp := refContentPath(ref)

	var transactionDoc *StorageObject
	var refInTransactions bool
	if r.InTransaction {
		for _, action := range r.TransactionActions {
			if action.Path == rp {
				transactionDoc = action.Doc
				refInTransactions = true
			}
		}
	}
	if refInTransactions {
		return transactionDoc, nil
	}

	results, err := r.Backend.Get(ctx, []string{
		rp,
	})
	if err != nil {
		return nil, err
	}
	if len(results) != 1 {
		return nil, nil
	}

	return results[0].Doc, nil
}

// GetLinks implements Store.
func (r *RefStore) GetLinks(ctx context.Context, ref string) ([]string, error) {
	doc, err := r.getRef(ctx, ref)
	if err != nil {
		return nil, err
	}
	if doc == nil {
		return nil, ErrRefNotFound
	}
	return doc.Links, nil
}

// Link implements Store.
func (r *RefStore) Link(ctx context.Context, ref string, target string) error {
	var requests []SetRequest

	targetDoc, err := r.getRef(ctx, target)
	if err != nil {
		return fmt.Errorf("get target %v: %w", target, err)
	}
	if targetDoc == nil {
		return ErrRefNotFound
	}

	existing, err := r.getRef(ctx, ref)
	if err != nil {
		return fmt.Errorf("load existing: %w", err)
	}

	if existing != nil && existing.Kind == "link" {
		unlinkReqs, err := r.unlinkRequests(ctx, ref)
		if err != nil {
			return fmt.Errorf("failed to unlink: %w", err)
		}
		requests = append(requests, unlinkReqs...)
	}

	targetJSON, err := json.Marshal(target)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}

	requests = append(requests, SetRequest{
		Path: refContentPath(ref),
		Doc: &StorageObject{
			Kind: "link",
			Body: targetJSON,
		},
	})

	targetDoc.Links = append(targetDoc.Links, ref)
	requests = append(requests, SetRequest{
		Path: refContentPath(target),
		Doc:  targetDoc,
	})
	err = r.handleRequest(ctx, requests...)
	if err != nil {
		return fmt.Errorf("applying link: %w", err)
	}
	return nil
}

// Match implements Store.
func (r *RefStore) Match(ctx context.Context, glob ...string) ([]string, error) {
	return r.MatchOptions(ctx, MatchOptions{}, glob...)
}

// MatchOptions implements Store.
func (r *RefStore) MatchOptions(ctx context.Context, options MatchOptions, glob ...string) ([]string, error) {
	var requests []MatchRequest
	for _, g := range glob {
		requests = append(requests, MatchRequest{
			Prefix:   refsDir + "/",
			Suffixes: []string{"/" + contentFile},
			Glob:     g,
		})
	}
	results, err := r.Backend.Match(ctx, requests)
	if err != nil {
		return nil, err
	}

	if options.NoLinks {
		var out []string
		for _, res := range results {
			doc, err := r.getRef(ctx, res)
			if err != nil {
				return nil, err
			}
			if doc == nil {
				continue
			}
			if doc.Kind == "link" {
				continue
			}
			out = append(out, res)
		}
		results = out
	}

	if r.InTransaction {
		var currentResults = make(map[string]struct{})
		for _, res := range results {
			currentResults[res] = struct{}{}
		}
		for _, action := range r.TransactionActions {
			var match bool
			for _, request := range requests {
				if request.Matches(action.Path) {
					match = true
					break
				}
			}
			if !match {
				continue
			}

			if action.Doc == nil {
				delete(currentResults, action.Path)
			} else {
				currentResults[action.Path] = struct{}{}
			}
		}
		results = make([]string, 0, len(currentResults))
		for res := range currentResults {
			results = append(results, res)
		}
	}

	var out []string
	for _, res := range results {
		res = strings.TrimPrefix(res, refsDir+"/")
		res = strings.TrimSuffix(res, "/"+contentFile)
		out = append(out, res)
	}
	results = out

	sort.Strings(out)

	return results, nil
}

// ResolveLink implements Store.
func (r *RefStore) ResolveLink(ctx context.Context, ref string) (string, error) {
	out, err := r.refOrLinkTarget(ctx, ref)
	if err != nil {
		return "", fmt.Errorf("resolving %v: %w", ref, err)
	}
	if out != "" {
		return out, nil
	}

	var prefix string
	dirs := strings.Split(ref, "/")
	for i := 0; i < len(dirs); i++ {
		if dirs[i] == "" {
			continue
		}
		prefix = path.Join(prefix, dirs[i])
		out, err := r.refOrLinkTarget(ctx, prefix)
		if err != nil {
			return "", fmt.Errorf("resolving %v: %w", prefix, err)
		}
		if out != "" && out != prefix {
			return path.Join(out, strings.Join(dirs[i+1:], "/")), nil
		}
	}

	return ref, nil
}

func (r *RefStore) refOrLinkTarget(ctx context.Context, ref string) (string, error) {
	result, err := r.getRef(ctx, ref)
	if err != nil {
		return "", err
	}
	if result == nil {
		return "", nil
	}
	if result.Kind != "link" {
		return ref, nil
	}
	var linkRef string
	err = json.Unmarshal(result.Body, &linkRef)
	if err != nil {
		return "", fmt.Errorf("unmarshal: %w", err)
	}

	return linkRef, nil
}

// Set implements Store.
func (r *RefStore) Set(ctx context.Context, ref string, v any) error {
	var doc StorageObject = StorageObject{
		Kind:     StorageKindRef,
		BodyType: fmt.Sprintf("%T", v),
	}

	// Retrieve the previous doc to preserve links, etc
	previous, err := r.getRef(ctx, ref)
	if err != nil {
		return err
	}
	if previous != nil {
		doc = *previous
	}

	if v != nil {
		var err error
		doc.Body, err = json.Marshal(v)
		if err != nil {
			return err
		}
	}

	return r.handleRequest(ctx, SetRequest{
		Path: refContentPath(ref),
		Doc:  &doc,
	})
}

// Unlink implements Store.
func (r *RefStore) Unlink(ctx context.Context, ref string) error {
	reqs, err := r.unlinkRequests(ctx, ref)
	if err != nil {
		return err
	}

	return r.handleRequest(ctx, reqs...)
}

func (r *RefStore) unlinkRequests(ctx context.Context, ref string) ([]SetRequest, error) {
	target, err := r.ResolveLink(ctx, ref)
	if err != nil {
		return nil, err
	}

	targetDoc, err := r.getRef(ctx, target)
	if err != nil {
		return nil, err
	}
	if targetDoc == nil {
		return nil, ErrRefNotFound
	}
	var filteredLinks []string
	for _, link := range targetDoc.Links {
		if link != ref {
			filteredLinks = append(filteredLinks, link)
		}
	}
	targetDoc.Links = filteredLinks

	return []SetRequest{
		{
			Path: refContentPath(ref),
			Doc:  nil,
		},
		{
			Path: refContentPath(target),
			Doc:  targetDoc,
		},
	}, nil
}
