// Package pricing provides a lightweight, embedded on-demand pricing table
// for estimating S3 storage costs. Estimates only -- real costs vary by
// storage class (Standard-IA, Glacier, etc.), request and data-transfer
// charges, and AWS's own price changes over time.
package pricing

// s3StandardPerGBMonth holds approximate on-demand S3 Standard storage
// pricing in USD per GB-month, by region, as of this package's last update.
var s3StandardPerGBMonth = map[string]float64{
	"us-east-1":      0.023,
	"us-east-2":      0.023,
	"us-west-1":      0.026,
	"us-west-2":      0.023,
	"eu-west-1":      0.024,
	"eu-west-2":      0.024,
	"eu-west-3":      0.024,
	"eu-central-1":   0.0245,
	"eu-north-1":     0.0221,
	"ap-southeast-1": 0.025,
	"ap-southeast-2": 0.025,
	"ap-northeast-1": 0.025,
	"ap-south-1":     0.025,
	"sa-east-1":      0.0405,
	"ca-central-1":   0.025,
}

const (
	defaultRegion = "us-east-1"
	bytesPerGiB   = 1024 * 1024 * 1024
)

// MonthlyStorageCost estimates the monthly USD cost of storing sizeBytes in
// S3 Standard storage in the given region. Falls back to us-east-1 pricing
// if the region is unknown or empty.
func MonthlyStorageCost(sizeBytes int64, region string) float64 {
	if sizeBytes <= 0 {
		return 0
	}
	price, ok := s3StandardPerGBMonth[region]
	if !ok {
		price = s3StandardPerGBMonth[defaultRegion]
	}
	gib := float64(sizeBytes) / bytesPerGiB
	return gib * price
}
