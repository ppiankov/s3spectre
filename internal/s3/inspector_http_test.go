package s3

import (
	"context"
	"io"
	"net/http"
	"sort"
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

func xmlErrorResponse(code, message string) *http.Response {
	body := `<?xml version="1.0" encoding="UTF-8"?>
<Error>
  <Code>` + code + `</Code>
  <Message>` + message + `</Message>
  <BucketName>test-bucket</BucketName>
  <RequestId>req-1</RequestId>
  <HostId>host-1</HostId>
</Error>`
	return &http.Response{
		StatusCode: http.StatusNotFound,
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

func TestInspector_FetchEncryption_Enabled(t *testing.T) {
	encXML := `<?xml version="1.0" encoding="UTF-8"?>
<ServerSideEncryptionConfiguration xmlns="http://s3.amazonaws.com/doc/2006-03-01/">
  <Rule>
    <ApplyServerSideEncryptionByDefault>
      <SSEAlgorithm>aws:kms</SSEAlgorithm>
      <KMSMasterKeyID>arn:aws:kms:us-east-1:123456789012:key/abcd</KMSMasterKeyID>
    </ApplyServerSideEncryptionByDefault>
  </Rule>
</ServerSideEncryptionConfiguration>`

	rt := roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		return xmlResponse(encXML), nil
	})
	client := newTestClient(t, rt)
	inspector := NewInspector(client, 1)

	enc := inspector.fetchEncryption(context.Background(), client, "test-bucket")
	if !enc.Enabled {
		t.Fatalf("expected encryption enabled")
	}
	if enc.Algorithm != "aws:kms" {
		t.Fatalf("expected algorithm aws:kms, got %q", enc.Algorithm)
	}
	if enc.KMSMasterKeyID == "" {
		t.Fatalf("expected KMS key id to be populated")
	}
}

func TestInspector_FetchEncryption_NotConfigured(t *testing.T) {
	rt := roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		return xmlErrorResponse("ServerSideEncryptionConfigurationNotFoundError", "not found"), nil
	})
	client := newTestClient(t, rt)
	inspector := NewInspector(client, 1)

	enc := inspector.fetchEncryption(context.Background(), client, "test-bucket")
	if enc == nil {
		t.Fatalf("expected non-nil EncryptionInfo even when not configured")
	}
	if enc.Enabled {
		t.Fatalf("expected encryption disabled when not configured")
	}
}

func TestInspector_FetchPublicAccessBlock_FullyBlocked(t *testing.T) {
	pabXML := `<?xml version="1.0" encoding="UTF-8"?>
<PublicAccessBlockConfiguration xmlns="http://s3.amazonaws.com/doc/2006-03-01/">
  <BlockPublicAcls>true</BlockPublicAcls>
  <IgnorePublicAcls>true</IgnorePublicAcls>
  <BlockPublicPolicy>true</BlockPublicPolicy>
  <RestrictPublicBuckets>true</RestrictPublicBuckets>
</PublicAccessBlockConfiguration>`

	rt := roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		return xmlResponse(pabXML), nil
	})
	client := newTestClient(t, rt)
	inspector := NewInspector(client, 1)

	pa := inspector.fetchPublicAccessBlock(context.Background(), client, "test-bucket")
	if pa.IsPublic {
		t.Fatalf("expected IsPublic=false when all four protections are enabled")
	}
}

func TestInspector_FetchPublicAccessBlock_PartiallyBlocked(t *testing.T) {
	pabXML := `<?xml version="1.0" encoding="UTF-8"?>
<PublicAccessBlockConfiguration xmlns="http://s3.amazonaws.com/doc/2006-03-01/">
  <BlockPublicAcls>true</BlockPublicAcls>
  <IgnorePublicAcls>true</IgnorePublicAcls>
  <BlockPublicPolicy>false</BlockPublicPolicy>
  <RestrictPublicBuckets>false</RestrictPublicBuckets>
</PublicAccessBlockConfiguration>`

	rt := roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		return xmlResponse(pabXML), nil
	})
	client := newTestClient(t, rt)
	inspector := NewInspector(client, 1)

	pa := inspector.fetchPublicAccessBlock(context.Background(), client, "test-bucket")
	if !pa.IsPublic {
		t.Fatalf("expected IsPublic=true when block-public-policy and restrict-public-buckets are both off")
	}
}

func TestInspector_FetchPublicAccessBlock_NotConfigured(t *testing.T) {
	rt := roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		return xmlErrorResponse("NoSuchPublicAccessBlockConfiguration", "not found"), nil
	})
	client := newTestClient(t, rt)
	inspector := NewInspector(client, 1)

	pa := inspector.fetchPublicAccessBlock(context.Background(), client, "test-bucket")
	if !pa.IsPublic {
		t.Fatalf("expected IsPublic=true when no public-access-block configuration exists at all")
	}
	if pa.BlockPublicAcls || pa.IgnorePublicAcls || pa.BlockPublicPolicy || pa.RestrictPublicBuckets {
		t.Fatalf("expected all four protection flags false when not configured, got %+v", pa)
	}
}

func listBucketsAndLocationsRoundTripper(locations map[string]string) http.RoundTripper {
	var names []string
	for name := range locations {
		names = append(names, name)
	}
	sort.Strings(names)

	var bucketsXML strings.Builder
	for _, name := range names {
		bucketsXML.WriteString("<Bucket><Name>" + name + "</Name><CreationDate>2024-01-01T00:00:00.000Z</CreationDate></Bucket>")
	}
	listBucketsXML := `<?xml version="1.0" encoding="UTF-8"?>
<ListAllMyBucketsResult xmlns="http://s3.amazonaws.com/doc/2006-03-01/">
  <Owner><ID>owner</ID><DisplayName>owner</DisplayName></Owner>
  <Buckets>` + bucketsXML.String() + `</Buckets>
</ListAllMyBucketsResult>`

	return roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		if strings.Contains(req.URL.RawQuery, "location") {
			bucket := strings.Trim(req.URL.Path, "/")
			loc := locations[bucket]
			body := `<?xml version="1.0" encoding="UTF-8"?>
<LocationConstraint xmlns="http://s3.amazonaws.com/doc/2006-03-01/">` + loc + `</LocationConstraint>`
			return xmlResponse(body), nil
		}
		return xmlResponse(listBucketsXML), nil
	})
}

func TestInspector_ListAllBucketsWithMetadata_FiltersByRegion(t *testing.T) {
	locations := map[string]string{
		"bucket-a": "eu-central-1",
		"bucket-b": "us-west-1",
	}
	client := newTestClient(t, listBucketsAndLocationsRoundTripper(locations))
	inspector := NewInspector(client, 1)

	buckets, bucketRegions, _, err := inspector.listAllBucketsWithMetadata(context.Background(), []string{"eu-central-1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(buckets) != 1 {
		t.Fatalf("expected 1 bucket after filtering to eu-central-1, got %d: %+v", len(buckets), buckets)
	}
	if !buckets["bucket-a"] {
		t.Fatalf("expected bucket-a to survive the eu-central-1 filter, got %+v", buckets)
	}
	if bucketRegions["bucket-a"] != "eu-central-1" {
		t.Fatalf("expected bucket-a region eu-central-1, got %q", bucketRegions["bucket-a"])
	}
}

func TestInspector_ListAllBucketsWithMetadata_NoFilterWhenRegionsEmpty(t *testing.T) {
	locations := map[string]string{
		"bucket-a": "eu-central-1",
		"bucket-b": "us-west-1",
	}
	client := newTestClient(t, listBucketsAndLocationsRoundTripper(locations))
	inspector := NewInspector(client, 1)

	buckets, _, _, err := inspector.listAllBucketsWithMetadata(context.Background(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(buckets) != 2 {
		t.Fatalf("expected both buckets with no region filter, got %d: %+v", len(buckets), buckets)
	}
}
