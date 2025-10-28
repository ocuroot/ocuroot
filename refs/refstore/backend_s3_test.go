//go:build integration

package refstore

import (
	"context"
	"os"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/joho/godotenv"
)

func TestS3Backend(t *testing.T) {
	err := godotenv.Load("../../.env")
	if err != nil {
		t.Fatalf("Loading .env file: %v", err)
	}

	endpoint := os.Getenv("S3_ENDPOINT")
	if endpoint == "" {
		t.Fatal("S3_ENDPOINT not set")
	}
	region := os.Getenv("S3_REGION")
	if region == "" {
		t.Fatal("S3_REGION not set")
	}
	accessKey := os.Getenv("S3_ACCESS_KEY")
	if accessKey == "" {
		t.Fatal("S3_ACCESS_KEY not set")
	}
	secretKey := os.Getenv("S3_SECRET_KEY")
	if secretKey == "" {
		t.Fatal("S3_SECRET_KEY not set")
	}
	bucket := os.Getenv("S3_TEST_BUCKET")
	if bucket == "" {
		t.Fatal("S3_TEST_BUCKET not set")
	}

	makeBackend := func() DocumentBackend {
		client := setupClient(endpoint, region, accessKey, secretKey)
		emptyBucket(t, client, bucket)
		be := &s3Backend{
			client: client,
			bucket: bucket,
		}
		return be
	}

	doTestBackendSetGet(t, makeBackend)
	doTestBackendMatch(t, makeBackend)
	doTestBackendInfo(t, makeBackend)
}

func setupClient(endpoint string, region string, accessKey string, secretKey string) *s3.Client {
	client := s3.New(s3.Options{
		EndpointResolverV2: s3.NewDefaultEndpointResolverV2(),
		Credentials:        credentials.NewStaticCredentialsProvider(accessKey, secretKey, ""),
		Region:             region,
		BaseEndpoint:       &endpoint,
	})
	return client
}

// empty a test bucket for a new test
func emptyBucket(t *testing.T, client *s3.Client, bucket string) {
	allObjects, err := client.ListObjectsV2(context.Background(), &s3.ListObjectsV2Input{
		Bucket: aws.String(bucket),
	})
	if err != nil {
		t.Fatal(err)
	}

	req := &s3.DeleteObjectsInput{
		Bucket: aws.String(bucket),
		Delete: &types.Delete{},
	}

	// If no objects, nothing to do
	if len(allObjects.Contents) == 0 {
		return
	}

	for _, obj := range allObjects.Contents {
		req.Delete.Objects = append(req.Delete.Objects, types.ObjectIdentifier{
			Key: obj.Key,
		})
	}

	_, err = client.DeleteObjects(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
}
