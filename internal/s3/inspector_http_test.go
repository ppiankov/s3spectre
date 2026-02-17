package s3

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func newTestClient(t *testing.T, rt http.RoundTripper) *Client {
	t.Helper()
	cfg := aws.Config{
		Region:      "us-east-1",
		Credentials: aws.NewCredentialsCache(credentials.NewStaticCredentialsProvider("AKID", "SECRET", "")),
		HTTPClient:  &http.Client{Transport: rt},
	}
	client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.UsePathStyle = true
		o.BaseEndpoint = aws.String("https://s3.us-east-1.amazonaws.com")
	})

	return &Client{s3Client: client, config: cfg}
}

func xmlResponse(body string) *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/xml"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func TestInspector_InspectPrefixWithClient(t *testing.T) {
	listObjectsXML := `<?xml version="1.0" encoding="UTF-8"?>
<ListBucketResult xmlns="http://s3.amazonaws.com/doc/2006-03-01/">
  <Name>test-bucket</Name>
  <Prefix>prefix/</Prefix>
  <KeyCount>2</KeyCount>
  <MaxKeys>1000</MaxKeys>
  <IsTruncated>false</IsTruncated>
  <Contents>
    <Key>prefix/file1</Key>
    <LastModified>2024-01-01T00:00:00.000Z</LastModified>
    <Size>10</Size>
  </Contents>
  <Contents>
    <Key>prefix/file2</Key>
    <LastModified>2024-01-02T00:00:00.000Z</LastModified>
    <Size>20</Size>
  </Contents>
</ListBucketResult>`

	rt := roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		return xmlResponse(listObjectsXML), nil
	})
	client := newTestClient(t, rt)
	inspector := NewInspector(client, 1)

	info := inspector.inspectPrefixWithClient(context.Background(), client, "test-bucket", "prefix/")
	if !info.Exists {
		t.Fatalf("expected prefix to exist")
	}
	if info.ObjectCount != 2 {
		t.Fatalf("expected object count 2, got %d", info.ObjectCount)
	}
	if info.LatestModified == nil {
		t.Fatalf("expected latest modified timestamp")
	}
	if info.DaysSinceModified <= 0 {
		t.Fatalf("expected days since modified to be > 0, got %d", info.DaysSinceModified)
	}
}

func TestInspector_InspectPrefixWithClient_Empty(t *testing.T) {
	listObjectsXML := `<?xml version="1.0" encoding="UTF-8"?>
<ListBucketResult xmlns="http://s3.amazonaws.com/doc/2006-03-01/">
  <Name>test-bucket</Name>
  <Prefix>empty/</Prefix>
  <KeyCount>0</KeyCount>
  <MaxKeys>1000</MaxKeys>
  <IsTruncated>false</IsTruncated>
</ListBucketResult>`

	rt := roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		return xmlResponse(listObjectsXML), nil
	})
	client := newTestClient(t, rt)
	inspector := NewInspector(client, 1)

	info := inspector.inspectPrefixWithClient(context.Background(), client, "test-bucket", "empty/")
	if info.Exists {
		t.Fatalf("expected prefix to be empty")
	}
	if info.ObjectCount != 0 {
		t.Fatalf("expected object count 0, got %d", info.ObjectCount)
	}
}

func TestInspector_CalculateVersionSizes(t *testing.T) {
	listVersionsXML := `<?xml version="1.0" encoding="UTF-8"?>
<ListVersionsResult xmlns="http://s3.amazonaws.com/doc/2006-03-01/">
  <Name>test-bucket</Name>
  <IsTruncated>false</IsTruncated>
  <Version>
    <Key>file1</Key>
    <VersionId>v1</VersionId>
    <IsLatest>true</IsLatest>
    <LastModified>2024-01-01T00:00:00.000Z</LastModified>
    <Size>100</Size>
  </Version>
  <DeleteMarker>
    <Key>file2</Key>
    <VersionId>v2</VersionId>
    <IsLatest>false</IsLatest>
    <LastModified>2024-01-02T00:00:00.000Z</LastModified>
  </DeleteMarker>
</ListVersionsResult>`

	rt := roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		return xmlResponse(listVersionsXML), nil
	})
	client := newTestClient(t, rt)
	inspector := NewInspector(client, 1)

	info := &BucketInfo{}
	inspector.calculateVersionSizes(context.Background(), client, "test-bucket", info)
	if info.TotalVersionSize != 100 {
		t.Fatalf("expected total version size 100, got %d", info.TotalVersionSize)
	}
	if info.VersionCount != 2 {
		t.Fatalf("expected version count 2, got %d", info.VersionCount)
	}
}

func TestInspector_InspectPrefixesWithClient(t *testing.T) {
	listObjectsXML := `<?xml version="1.0" encoding="UTF-8"?>
<ListBucketResult xmlns="http://s3.amazonaws.com/doc/2006-03-01/">
  <Name>test-bucket</Name>
  <Prefix>any/</Prefix>
  <KeyCount>1</KeyCount>
  <MaxKeys>1000</MaxKeys>
  <IsTruncated>false</IsTruncated>
  <Contents>
    <Key>any/file</Key>
    <LastModified>2024-01-01T00:00:00.000Z</LastModified>
    <Size>10</Size>
  </Contents>
</ListBucketResult>`

	rt := roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		return xmlResponse(listObjectsXML), nil
	})
	client := newTestClient(t, rt)
	inspector := NewInspector(client, 2)

	prefixes := []string{"one/", "two/"}
	results := inspector.inspectPrefixesWithClient(context.Background(), client, "test-bucket", prefixes)
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	seen := map[string]bool{}
	for _, result := range results {
		seen[result.Prefix] = true
		if !result.Exists {
			t.Fatalf("expected prefix %s to exist", result.Prefix)
		}
		if result.ObjectCount != 1 {
			t.Fatalf("expected object count 1 for %s, got %d", result.Prefix, result.ObjectCount)
		}
	}
	for _, prefix := range prefixes {
		if !seen[prefix] {
			t.Fatalf("missing prefix %s in results", prefix)
		}
	}
}
