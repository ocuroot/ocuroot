package refstore

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

var _ DocumentBackend = (*s3Backend)(nil)

type s3Backend struct {
	client *s3.Client
	bucket string
	prefix string
}

// Get implements DocumentBackend.
func (s *s3Backend) Get(ctx context.Context, paths []string) ([]GetResult, error) {
	var out []GetResult
	for _, path := range paths {
		obj, err := s.getObjectOrNil(ctx, path)
		if err != nil {
			return nil, err
		}
		if obj == nil {
			out = append(out, GetResult{
				Path: path,
			})
			continue
		}
		defer obj.Body.Close()
		body, err := io.ReadAll(obj.Body)
		if err != nil {
			return nil, err
		}

		var so StorageObject
		if err := json.Unmarshal(body, &so); err != nil {
			return nil, err
		}

		out = append(out, GetResult{
			Path: path,
			Doc:  &so,
		})
	}
	return out, nil
}

// GetBytes implements DocumentBackend.
func (s *s3Backend) GetBytes(ctx context.Context, path string) ([]byte, error) {
	obj, err := s.getObjectOrNil(ctx, path)
	if err != nil {
		return nil, err
	}
	if obj == nil {
		return nil, nil
	}
	defer obj.Body.Close()
	body, err := io.ReadAll(obj.Body)
	if err != nil {
		return nil, err
	}
	return body, nil
}

// Marker implements DocumentBackend.
func (s *s3Backend) Marker() ([]byte, error) {
	return nil, nil
}

// Match implements DocumentBackend.
func (s *s3Backend) Match(ctx context.Context, reqs []MatchRequest) ([]string, error) {
	compiledReqs, err := compileMatchRequests(reqs)
	if err != nil {
		return nil, err
	}

	pathsByPrefix := make(map[string][]string)
	for _, req := range compiledReqs {
		if _, exists := pathsByPrefix[req.prefix]; exists {
			continue
		}

		hasMore := true
		var continuationToken *string

		for hasMore {
			result, err := s.client.ListObjectsV2(context.Background(), &s3.ListObjectsV2Input{
				Bucket:            aws.String(s.bucket),
				ContinuationToken: continuationToken,
				Prefix:            aws.String(req.prefix),
			})
			if err != nil {
				return nil, err
			}

			for _, obj := range result.Contents {
				pathsByPrefix[req.prefix] = append(pathsByPrefix[req.prefix], *obj.Key)
			}

			hasMore = *result.IsTruncated
			continuationToken = result.NextContinuationToken
		}
	}

	var outPaths = make(map[string]struct{})
	for _, req := range compiledReqs {
		allPaths := pathsByPrefix[req.prefix]

		for _, p := range allPaths {
			trimmedP := strings.TrimPrefix(p, req.prefix)

			var hasSuffix bool
			suffixes := req.suffixes
			if len(suffixes) == 0 {
				suffixes = []string{""}
			}
			for _, suffix := range suffixes {
				if strings.HasSuffix(trimmedP, suffix) {
					hasSuffix = true
					trimmedP = trimmedP[:len(trimmedP)-len(suffix)]
					break
				}
			}
			if !hasSuffix {
				continue
			}

			if req.compiledGlob.Match(trimmedP) {
				outPaths[p] = struct{}{}
			}
		}
	}

	var out []string
	for p := range outPaths {
		out = append(out, p)
	}
	sort.Strings(out)

	return out, nil
}

// Set implements DocumentBackend.
func (s *s3Backend) Set(ctx context.Context, marker []byte, message string, reqs []SetRequest) error {
	for _, req := range reqs {
		if req.Doc == nil {
			_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
				Bucket: aws.String(s.bucket),
				Key:    aws.String(s.prefix + req.Path),
			})
			if err != nil {
				return fmt.Errorf("unhandled error: %w", err)
			}
			continue
		}

		bodyJSON, err := json.MarshalIndent(req.Doc, "", "  ")
		if err != nil {
			return fmt.Errorf("marshal: %w", err)
		}

		bodyReader := bytes.NewReader(bodyJSON)
		_, err = s.client.PutObject(ctx, &s3.PutObjectInput{
			Bucket: aws.String(s.bucket),
			Key:    aws.String(s.prefix + req.Path),
			Body:   bodyReader,
		})
		if err != nil {
			return fmt.Errorf("unhandled error: %w", err)
		}
	}
	return nil
}

// SetBytes implements DocumentBackend.
func (s *s3Backend) SetBytes(ctx context.Context, path string, content []byte) error {
	bodyReader := bytes.NewReader(content)
	_, err := s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(s.prefix + path),
		Body:   bodyReader,
	})
	if err != nil {
		return fmt.Errorf("unhandled error: %w", err)
	}
	return nil
}

func (s *s3Backend) getObjectOrNil(ctx context.Context, path string) (*s3.GetObjectOutput, error) {
	obj, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(s.prefix + path),
	})
	var notFound *types.NoSuchKey
	if errors.As(err, &notFound) {
		return nil, nil
	} else if err != nil {
		return nil, fmt.Errorf("unhandled error: %w", err)
	}
	return obj, err
}
