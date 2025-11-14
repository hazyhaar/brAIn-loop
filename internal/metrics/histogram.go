package metrics

import (
	"database/sql"
	"fmt"
	"math"
	"sort"
	"time"
)

// LatencyBuckets defines histogram buckets in milliseconds
var LatencyBuckets = []int{10, 50, 100, 500, 1000, 5000, 10000}

// Histogram manages latency histogram data
type Histogram struct {
	db *sql.DB
}

// NewHistogram creates a new histogram manager
func NewHistogram(db *sql.DB) *Histogram {
	return &Histogram{db: db}
}

// RecordLatency records a latency measurement in the histogram
func (h *Histogram) RecordLatency(operation string, latencyMs int) error {
	bucket := findBucket(latencyMs)
	timestamp := time.Now().Unix() / 60 * 60 // 1-minute windows

	_, err := h.db.Exec(`
		INSERT INTO latency_histogram (operation, bucket_ms, count, timestamp)
		VALUES (?, ?, 1, ?)
		ON CONFLICT(operation, bucket_ms, timestamp)
		DO UPDATE SET count = count + 1
	`, operation, bucket, timestamp)

	return err
}

// findBucket finds the appropriate bucket for a latency value
func findBucket(latencyMs int) int {
	for _, bucket := range LatencyBuckets {
		if latencyMs <= bucket {
			return bucket
		}
	}
	return LatencyBuckets[len(LatencyBuckets)-1]
}

// Percentiles holds calculated percentile values
type Percentiles struct {
	Operation string
	P50       float64
	P95       float64
	P99       float64
	Count     int
	WindowEnd int64
}

// CalculatePercentiles calculates p50, p95, p99 for an operation
func (h *Histogram) CalculatePercentiles(operation string, windowMinutes int) (*Percentiles, error) {
	windowStart := time.Now().Unix()/60*60 - int64(windowMinutes*60)

	rows, err := h.db.Query(`
		SELECT bucket_ms, SUM(count) as total_count
		FROM latency_histogram
		WHERE operation = ? AND timestamp >= ?
		GROUP BY bucket_ms
		ORDER BY bucket_ms ASC
	`, operation, windowStart)
	if err != nil {
		return nil, fmt.Errorf("failed to query histogram: %w", err)
	}
	defer rows.Close()

	type bucketData struct {
		bucket int
		count  int
	}

	var buckets []bucketData
	totalCount := 0

	for rows.Next() {
		var bd bucketData
		if err := rows.Scan(&bd.bucket, &bd.count); err != nil {
			return nil, err
		}
		buckets = append(buckets, bd)
		totalCount += bd.count
	}

	if totalCount == 0 {
		return nil, fmt.Errorf("no data available for operation %s", operation)
	}

	// Calculate percentiles
	p50 := calculatePercentile(buckets, totalCount, 0.50)
	p95 := calculatePercentile(buckets, totalCount, 0.95)
	p99 := calculatePercentile(buckets, totalCount, 0.99)

	return &Percentiles{
		Operation: operation,
		P50:       p50,
		P95:       p95,
		P99:       p99,
		Count:     totalCount,
		WindowEnd: time.Now().Unix(),
	}, nil
}

// calculatePercentile calculates a specific percentile from bucket data
func calculatePercentile(buckets []bucketData, totalCount int, percentile float64) float64 {
	if len(buckets) == 0 || totalCount == 0 {
		return 0
	}

	targetCount := int(math.Ceil(float64(totalCount) * percentile))
	cumulativeCount := 0

	for _, bd := range buckets {
		cumulativeCount += bd.count
		if cumulativeCount >= targetCount {
			// Linear interpolation within bucket
			prevCumulative := cumulativeCount - bd.count
			ratio := float64(targetCount-prevCumulative) / float64(bd.count)

			// Find previous bucket value
			prevBucket := 0
			for i, b := range LatencyBuckets {
				if b == bd.bucket && i > 0 {
					prevBucket = LatencyBuckets[i-1]
					break
				}
			}

			// Interpolate
			return float64(prevBucket) + ratio*float64(bd.bucket-prevBucket)
		}
	}

	// Fallback to highest bucket
	return float64(buckets[len(buckets)-1].bucket)
}

// GetAllPercentiles returns percentiles for all operations
func (h *Histogram) GetAllPercentiles(windowMinutes int) (map[string]*Percentiles, error) {
	// Get unique operations
	rows, err := h.db.Query(`
		SELECT DISTINCT operation
		FROM latency_histogram
		WHERE timestamp >= ?
	`, time.Now().Unix()/60*60-int64(windowMinutes*60))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	results := make(map[string]*Percentiles)

	for rows.Next() {
		var operation string
		if err := rows.Scan(&operation); err != nil {
			continue
		}

		percentiles, err := h.CalculatePercentiles(operation, windowMinutes)
		if err != nil {
			continue
		}

		results[operation] = percentiles
	}

	return results, nil
}

// CleanupOldData removes histogram data older than retentionDays
func (h *Histogram) CleanupOldData(retentionDays int) (int64, error) {
	cutoff := time.Now().Unix() - int64(retentionDays*24*3600)

	result, err := h.db.Exec(`
		DELETE FROM latency_histogram
		WHERE timestamp < ?
	`, cutoff)
	if err != nil {
		return 0, err
	}

	return result.RowsAffected()
}

// HistogramSummary provides summary statistics
type HistogramSummary struct {
	Operation      string
	TotalSamples   int
	MinLatency     int
	MaxLatency     int
	AvgLatency     float64
	StdDevLatency  float64
	BucketCounts   map[int]int
	LastUpdated    int64
}

// GetSummary returns summary statistics for an operation
func (h *Histogram) GetSummary(operation string, windowMinutes int) (*HistogramSummary, error) {
	windowStart := time.Now().Unix()/60*60 - int64(windowMinutes*60)

	rows, err := h.db.Query(`
		SELECT bucket_ms, SUM(count) as total_count
		FROM latency_histogram
		WHERE operation = ? AND timestamp >= ?
		GROUP BY bucket_ms
		ORDER BY bucket_ms ASC
	`, operation, windowStart)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	bucketCounts := make(map[int]int)
	totalSamples := 0
	minLatency := math.MaxInt32
	maxLatency := 0
	sumLatency := 0.0
	sumSquares := 0.0

	for rows.Next() {
		var bucket, count int
		if err := rows.Scan(&bucket, &count); err != nil {
			return nil, err
		}

		bucketCounts[bucket] = count
		totalSamples += count

		// Approximate min/max/avg using bucket values
		if bucket < minLatency {
			minLatency = bucket
		}
		if bucket > maxLatency {
			maxLatency = bucket
		}

		// Weighted average (assume mid-bucket value)
		bucketValue := float64(bucket)
		sumLatency += bucketValue * float64(count)
		sumSquares += bucketValue * bucketValue * float64(count)
	}

	if totalSamples == 0 {
		return nil, fmt.Errorf("no data for operation %s", operation)
	}

	avgLatency := sumLatency / float64(totalSamples)
	variance := (sumSquares / float64(totalSamples)) - (avgLatency * avgLatency)
	stdDev := math.Sqrt(math.Max(0, variance))

	return &HistogramSummary{
		Operation:     operation,
		TotalSamples:  totalSamples,
		MinLatency:    minLatency,
		MaxLatency:    maxLatency,
		AvgLatency:    avgLatency,
		StdDevLatency: stdDev,
		BucketCounts:  bucketCounts,
		LastUpdated:   time.Now().Unix(),
	}, nil
}

// GetTopOperations returns operations sorted by sample count
func (h *Histogram) GetTopOperations(windowMinutes, limit int) ([]string, error) {
	windowStart := time.Now().Unix()/60*60 - int64(windowMinutes*60)

	rows, err := h.db.Query(`
		SELECT operation, SUM(count) as total_count
		FROM latency_histogram
		WHERE timestamp >= ?
		GROUP BY operation
		ORDER BY total_count DESC
		LIMIT ?
	`, windowStart, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var operations []string
	for rows.Next() {
		var operation string
		var count int
		if err := rows.Scan(&operation, &count); err != nil {
			continue
		}
		operations = append(operations, operation)
	}

	return operations, nil
}

// BucketDistribution returns the distribution across buckets for an operation
type BucketDistribution struct {
	Bucket      int
	Count       int
	Percentage  float64
	Cumulative  float64
}

// GetBucketDistribution returns bucket distribution for an operation
func (h *Histogram) GetBucketDistribution(operation string, windowMinutes int) ([]BucketDistribution, error) {
	windowStart := time.Now().Unix()/60*60 - int64(windowMinutes*60)

	rows, err := h.db.Query(`
		SELECT bucket_ms, SUM(count) as total_count
		FROM latency_histogram
		WHERE operation = ? AND timestamp >= ?
		GROUP BY bucket_ms
		ORDER BY bucket_ms ASC
	`, operation, windowStart)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	type bucketCount struct {
		bucket int
		count  int
	}

	var buckets []bucketCount
	totalCount := 0

	for rows.Next() {
		var bc bucketCount
		if err := rows.Scan(&bc.bucket, &bc.count); err != nil {
			return nil, err
		}
		buckets = append(buckets, bc)
		totalCount += bc.count
	}

	if totalCount == 0 {
		return nil, fmt.Errorf("no data for operation %s", operation)
	}

	distribution := make([]BucketDistribution, len(buckets))
	cumulativeCount := 0

	for i, bc := range buckets {
		cumulativeCount += bc.count
		distribution[i] = BucketDistribution{
			Bucket:     bc.bucket,
			Count:      bc.count,
			Percentage: float64(bc.count) / float64(totalCount) * 100.0,
			Cumulative: float64(cumulativeCount) / float64(totalCount) * 100.0,
		}
	}

	return distribution, nil
}
