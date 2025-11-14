package metrics

import (
	"database/sql"
	"os"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

func setupTestDB(t *testing.T) *sql.DB {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open test database: %v", err)
	}

	// Create schema
	_, err = db.Exec(`
		CREATE TABLE latency_histogram (
			operation TEXT NOT NULL,
			bucket_ms INTEGER NOT NULL,
			count INTEGER DEFAULT 0,
			timestamp INTEGER NOT NULL,
			PRIMARY KEY (operation, bucket_ms, timestamp)
		)
	`)
	if err != nil {
		t.Fatalf("Failed to create schema: %v", err)
	}

	return db
}

func TestFindBucket(t *testing.T) {
	tests := []struct {
		latency  int
		expected int
	}{
		{5, 10},
		{10, 10},
		{25, 50},
		{150, 500},
		{999, 1000},
		{5001, 10000},
		{20000, 10000}, // Above max bucket
	}

	for _, tt := range tests {
		result := findBucket(tt.latency)
		if result != tt.expected {
			t.Errorf("findBucket(%d) = %d, expected %d", tt.latency, result, tt.expected)
		}
	}
}

func TestRecordLatency(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	h := NewHistogram(db)

	// Record latencies
	operations := []struct {
		op      string
		latency int
	}{
		{"read_file", 45},
		{"read_file", 55},
		{"read_file", 150},
		{"execute_bash", 2500},
		{"execute_bash", 3500},
	}

	for _, op := range operations {
		err := h.RecordLatency(op.op, op.latency)
		if err != nil {
			t.Fatalf("Failed to record latency: %v", err)
		}
	}

	// Verify bucket counts
	var count int
	err := db.QueryRow(`
		SELECT SUM(count) FROM latency_histogram WHERE operation = 'read_file'
	`).Scan(&count)
	if err != nil {
		t.Fatalf("Failed to query: %v", err)
	}

	if count != 3 {
		t.Errorf("Expected 3 samples for read_file, got %d", count)
	}
}

func TestCalculatePercentiles(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	h := NewHistogram(db)

	// Insert test data directly
	timestamp := time.Now().Unix() / 60 * 60

	testData := []struct {
		bucket int
		count  int
	}{
		{10, 5},    // 5 samples in 0-10ms bucket
		{50, 10},   // 10 samples in 10-50ms bucket
		{100, 30},  // 30 samples in 50-100ms bucket
		{500, 45},  // 45 samples in 100-500ms bucket
		{1000, 10}, // 10 samples in 500-1000ms bucket
	}

	for _, td := range testData {
		_, err := db.Exec(`
			INSERT INTO latency_histogram (operation, bucket_ms, count, timestamp)
			VALUES (?, ?, ?, ?)
		`, "test_op", td.bucket, td.count, timestamp)
		if err != nil {
			t.Fatalf("Failed to insert test data: %v", err)
		}
	}

	// Calculate percentiles
	percentiles, err := h.CalculatePercentiles("test_op", 60)
	if err != nil {
		t.Fatalf("Failed to calculate percentiles: %v", err)
	}

	// Total: 100 samples
	// p50 (50th sample): should be in 100ms bucket
	// p95 (95th sample): should be in 500ms bucket
	// p99 (99th sample): should be in 500ms bucket

	if percentiles.Count != 100 {
		t.Errorf("Expected count=100, got %d", percentiles.Count)
	}

	if percentiles.P50 < 50 || percentiles.P50 > 150 {
		t.Errorf("P50 out of expected range: %f", percentiles.P50)
	}

	if percentiles.P95 < 300 || percentiles.P95 > 600 {
		t.Errorf("P95 out of expected range: %f", percentiles.P95)
	}
}

func TestGetSummary(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	h := NewHistogram(db)
	timestamp := time.Now().Unix() / 60 * 60

	// Insert test data
	db.Exec(`INSERT INTO latency_histogram VALUES (?, 50, 10, ?)`, "test_op", timestamp)
	db.Exec(`INSERT INTO latency_histogram VALUES (?, 100, 20, ?)`, "test_op", timestamp)
	db.Exec(`INSERT INTO latency_histogram VALUES (?, 500, 5, ?)`, "test_op", timestamp)

	summary, err := h.GetSummary("test_op", 60)
	if err != nil {
		t.Fatalf("Failed to get summary: %v", err)
	}

	if summary.TotalSamples != 35 {
		t.Errorf("Expected 35 total samples, got %d", summary.TotalSamples)
	}

	if summary.MinLatency != 50 {
		t.Errorf("Expected min=50, got %d", summary.MinLatency)
	}

	if summary.MaxLatency != 500 {
		t.Errorf("Expected max=500, got %d", summary.MaxLatency)
	}

	if len(summary.BucketCounts) != 3 {
		t.Errorf("Expected 3 buckets, got %d", len(summary.BucketCounts))
	}
}

func TestGetBucketDistribution(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	h := NewHistogram(db)
	timestamp := time.Now().Unix() / 60 * 60

	// Insert test data (100 total samples)
	db.Exec(`INSERT INTO latency_histogram VALUES (?, 10, 20, ?)`, "test_op", timestamp)
	db.Exec(`INSERT INTO latency_histogram VALUES (?, 50, 30, ?)`, "test_op", timestamp)
	db.Exec(`INSERT INTO latency_histogram VALUES (?, 100, 50, ?)`, "test_op", timestamp)

	distribution, err := h.GetBucketDistribution("test_op", 60)
	if err != nil {
		t.Fatalf("Failed to get distribution: %v", err)
	}

	if len(distribution) != 3 {
		t.Errorf("Expected 3 buckets, got %d", len(distribution))
	}

	// Check percentages
	if distribution[0].Percentage != 20.0 {
		t.Errorf("Expected 20%% for first bucket, got %f", distribution[0].Percentage)
	}

	// Check cumulative
	if distribution[2].Cumulative != 100.0 {
		t.Errorf("Expected 100%% cumulative for last bucket, got %f", distribution[2].Cumulative)
	}
}

func TestCleanupOldData(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	h := NewHistogram(db)

	// Insert old data (8 days ago)
	oldTimestamp := time.Now().Unix() - 8*24*3600
	db.Exec(`INSERT INTO latency_histogram VALUES (?, 50, 10, ?)`, "old_op", oldTimestamp)

	// Insert recent data (1 day ago)
	recentTimestamp := time.Now().Unix() - 1*24*3600
	db.Exec(`INSERT INTO latency_histogram VALUES (?, 50, 10, ?)`, "recent_op", recentTimestamp)

	// Cleanup data older than 7 days
	deleted, err := h.CleanupOldData(7)
	if err != nil {
		t.Fatalf("Failed to cleanup: %v", err)
	}

	if deleted != 1 {
		t.Errorf("Expected 1 row deleted, got %d", deleted)
	}

	// Verify old data is gone
	var count int
	db.QueryRow(`SELECT COUNT(*) FROM latency_histogram WHERE operation = 'old_op'`).Scan(&count)
	if count != 0 {
		t.Error("Old data not deleted")
	}

	// Verify recent data remains
	db.QueryRow(`SELECT COUNT(*) FROM latency_histogram WHERE operation = 'recent_op'`).Scan(&count)
	if count != 1 {
		t.Error("Recent data was deleted")
	}
}

func TestGetTopOperations(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	h := NewHistogram(db)
	timestamp := time.Now().Unix() / 60 * 60

	// Insert data for multiple operations
	db.Exec(`INSERT INTO latency_histogram VALUES (?, 50, 100, ?)`, "read_file", timestamp)
	db.Exec(`INSERT INTO latency_histogram VALUES (?, 50, 50, ?)`, "execute_bash", timestamp)
	db.Exec(`INSERT INTO latency_histogram VALUES (?, 50, 200, ?)`, "parse_code", timestamp)

	// Get top 2 operations
	top, err := h.GetTopOperations(60, 2)
	if err != nil {
		t.Fatalf("Failed to get top operations: %v", err)
	}

	if len(top) != 2 {
		t.Errorf("Expected 2 operations, got %d", len(top))
	}

	// Should be ordered by count DESC
	if top[0] != "parse_code" {
		t.Errorf("Expected parse_code first, got %s", top[0])
	}

	if top[1] != "read_file" {
		t.Errorf("Expected read_file second, got %s", top[1])
	}
}

func TestGetAllPercentiles(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	h := NewHistogram(db)
	timestamp := time.Now().Unix() / 60 * 60

	// Insert data for 2 operations
	db.Exec(`INSERT INTO latency_histogram VALUES (?, 50, 10, ?)`, "op1", timestamp)
	db.Exec(`INSERT INTO latency_histogram VALUES (?, 100, 20, ?)`, "op1", timestamp)
	db.Exec(`INSERT INTO latency_histogram VALUES (?, 100, 15, ?)`, "op2", timestamp)

	allPercentiles, err := h.GetAllPercentiles(60)
	if err != nil {
		t.Fatalf("Failed to get all percentiles: %v", err)
	}

	if len(allPercentiles) != 2 {
		t.Errorf("Expected 2 operations, got %d", len(allPercentiles))
	}

	if _, exists := allPercentiles["op1"]; !exists {
		t.Error("Missing op1 in results")
	}

	if _, exists := allPercentiles["op2"]; !exists {
		t.Error("Missing op2 in results")
	}
}
