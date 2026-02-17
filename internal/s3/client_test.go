package s3

import (
	"context"
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
)

func TestNewClient_RegionOverride(t *testing.T) {
	t.Setenv("AWS_ACCESS_KEY_ID", "test")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "test")
	t.Setenv("AWS_SESSION_TOKEN", "test")
	t.Setenv("AWS_EC2_METADATA_DISABLED", "true")
	t.Setenv("AWS_REGION", "us-west-2")

	client, err := NewClient(context.Background(), "", "us-east-1")
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}
	if client.GetRegion() != "us-east-1" {
		t.Fatalf("expected region us-east-1, got %q", client.GetRegion())
	}
	if client.GetClient() == nil {
		t.Fatalf("expected non-nil s3 client")
	}
	if client.GetConfig().Region != "us-east-1" {
		t.Fatalf("expected config region us-east-1, got %q", client.GetConfig().Region)
	}
}

func TestNewClientForRegion_CopiesConfig(t *testing.T) {
	base := aws.Config{Region: "us-west-2"}
	client := NewClientForRegion(base, "eu-central-1")

	if base.Region != "us-west-2" {
		t.Fatalf("expected base region to remain us-west-2, got %q", base.Region)
	}
	if client.GetRegion() != "eu-central-1" {
		t.Fatalf("expected client region eu-central-1, got %q", client.GetRegion())
	}
}

func TestIsRetryableError(t *testing.T) {
	tests := []struct {
		err      error
		retryable bool
	}{
		{errors.New("RequestLimitExceeded"), true},
		{errors.New("SlowDown"), true},
		{errors.New("InternalError"), true},
		{errors.New("status code: 429"), true},
		{errors.New("AccessDenied"), false},
		{errors.New("some other error"), false},
		{nil, false},
	}

	for _, tt := range tests {
		if got := isRetryableError(tt.err); got != tt.retryable {
			if tt.err == nil {
				t.Fatalf("expected retryable %v for nil error, got %v", tt.retryable, got)
			}
			t.Fatalf("expected retryable %v for %q, got %v", tt.retryable, tt.err.Error(), got)
		}
	}
}

func TestWithRetry_NonRetryable(t *testing.T) {
	client := &Client{}
	expectedErr := errors.New("AccessDenied")

	err := client.WithRetry(context.Background(), func() error {
		return expectedErr
	})
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if err.Error() != expectedErr.Error() {
		t.Fatalf("expected %q, got %q", expectedErr.Error(), err.Error())
	}
}

func TestWithRetry_ContextCanceled(t *testing.T) {
	client := &Client{}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := client.WithRetry(ctx, func() error {
		return errors.New("SlowDown")
	})
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context canceled, got %v", err)
	}
}
