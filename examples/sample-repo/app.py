#!/usr/bin/env python3
"""Sample application with S3 references."""

import boto3

# S3 bucket references
BUCKET_NAME = "my-app-data-prod"
BACKUP_BUCKET = "my-app-backups"
LOG_BUCKET = "app-logs-prod"

def upload_file(file_path, key):
    """Upload file to S3."""
    s3_client = boto3.client('s3')

    # Upload to primary bucket
    s3_client.upload_file(file_path, BUCKET_NAME, key)

    # Also backup
    s3_client.copy_object(
        Bucket=BACKUP_BUCKET,
        CopySource={'Bucket': BUCKET_NAME, 'Key': key},
        Key=key
    )

def get_log_url(log_file):
    """Get presigned URL for log file."""
    # Direct S3 URL
    return f"s3://{LOG_BUCKET}/logs/application/{log_file}"

def download_media(media_id):
    """Download media from S3."""
    s3_url = f"s3://user-uploads-prod/media/{media_id}"
    # Download logic here
    pass

if __name__ == "__main__":
    print("Sample app with S3 integration")
