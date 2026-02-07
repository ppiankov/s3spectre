terraform {
  required_version = ">= 1.0"
}

# Primary data bucket
resource "aws_s3_bucket" "app_data" {
  bucket = "my-app-data-prod"

  tags = {
    Environment = "production"
    Application = "sample-app"
  }
}

# Versioning for data bucket
resource "aws_s3_bucket_versioning" "app_data_versioning" {
  bucket = aws_s3_bucket.app_data.id

  versioning_configuration {
    status = "Enabled"
  }
}

# Backup bucket
resource "aws_s3_bucket" "backups" {
  bucket = "my-app-backups"

  tags = {
    Environment = "production"
    Purpose     = "backups"
  }
}

# Log bucket
resource "aws_s3_bucket" "logs" {
  bucket = "app-logs-prod"

  tags = {
    Environment = "production"
    Purpose     = "logging"
  }
}

# User uploads bucket
resource "aws_s3_bucket" "uploads" {
  bucket = "user-uploads-prod"

  tags = {
    Environment = "production"
    Purpose     = "user-content"
  }
}
