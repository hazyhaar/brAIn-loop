package database

import (
	"database/sql"
	"time"
)

// OutputDB provides helper methods for output database operations
type OutputDB struct {
	db *sql.DB
}

// NewOutputDB creates a new output database helper
func NewOutputDB(db *sql.DB) *OutputDB {
	return &OutputDB{db: db}
}

// PublishResult publishes a final result (committed session)
func (o *OutputDB) PublishResult(hash, sessionID string, blocksCommitted int, dataJSON string) error {
	_, err := o.db.Exec(`
		INSERT INTO results (hash, session_id, blocks_committed, data_json, created_at)
		VALUES (?, ?, ?, ?, ?)
	`, hash, sessionID, blocksCommitted, dataJSON, time.Now().Unix())
	return err
}

// GetResult retrieves a result by hash
func (o *OutputDB) GetResult(hash string) (map[string]interface{}, error) {
	var sessionID, dataJSON string
	var blocksCommitted int
	var createdAt int64

	err := o.db.QueryRow(`
		SELECT session_id, blocks_committed, data_json, created_at
		FROM results
		WHERE hash = ?
	`, hash).Scan(&sessionID, &blocksCommitted, &dataJSON, &createdAt)

	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"hash":             hash,
		"session_id":       sessionID,
		"blocks_committed": blocksCommitted,
		"data_json":        dataJSON,
		"created_at":       createdAt,
	}, nil
}

// PublishDigest publishes a reader digest
func (o *OutputDB) PublishDigest(hash, sourceType, sourcePath, digestJSON string) error {
	_, err := o.db.Exec(`
		INSERT OR REPLACE INTO reader_digests
		(hash, source_type, source_path, digest_json, created_at)
		VALUES (?, ?, ?, ?, ?)
	`, hash, sourceType, sourcePath, digestJSON, time.Now().Unix())
	return err
}

// GetDigest retrieves a digest by hash
func (o *OutputDB) GetDigest(hash string) (map[string]interface{}, error) {
	var sourceType, sourcePath, digestJSON string
	var createdAt int64

	err := o.db.QueryRow(`
		SELECT source_type, source_path, digest_json, created_at
		FROM reader_digests
		WHERE hash = ?
	`, hash).Scan(&sourceType, &sourcePath, &digestJSON, &createdAt)

	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"hash":        hash,
		"source_type": sourceType,
		"source_path": sourcePath,
		"digest_json": digestJSON,
		"created_at":  createdAt,
	}, nil
}

// RecordMetric records an observability metric
func (o *OutputDB) RecordMetric(metricName string, metricValue float64) error {
	_, err := o.db.Exec(`
		INSERT INTO metrics (timestamp, metric_name, metric_value)
		VALUES (?, ?, ?)
	`, time.Now().Unix(), metricName, metricValue)
	return err
}

// GetMetrics retrieves metrics within a time range
func (o *OutputDB) GetMetrics(metricName string, startTime, endTime int64) ([]map[string]interface{}, error) {
	rows, err := o.db.Query(`
		SELECT timestamp, metric_name, metric_value
		FROM metrics
		WHERE metric_name = ? AND timestamp >= ? AND timestamp <= ?
		ORDER BY timestamp ASC
	`, metricName, startTime, endTime)

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []map[string]interface{}
	for rows.Next() {
		var timestamp int64
		var name string
		var value float64

		if err := rows.Scan(&timestamp, &name, &value); err != nil {
			return nil, err
		}

		results = append(results, map[string]interface{}{
			"timestamp":    timestamp,
			"metric_name":  name,
			"metric_value": value,
		})
	}

	return results, rows.Err()
}

// GetAggregatedMetrics retrieves aggregated metrics
func (o *OutputDB) GetAggregatedMetrics(since int64) (map[string]interface{}, error) {
	rows, err := o.db.Query(`
		SELECT metric_name, COUNT(*) as count, AVG(metric_value) as avg, MAX(metric_value) as max, MIN(metric_value) as min
		FROM metrics
		WHERE timestamp >= ?
		GROUP BY metric_name
	`, since)

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	results := make(map[string]interface{})
	for rows.Next() {
		var name string
		var count int
		var avg, max, min float64

		if err := rows.Scan(&name, &count, &avg, &max, &min); err != nil {
			return nil, err
		}

		results[name] = map[string]interface{}{
			"count": count,
			"avg":   avg,
			"max":   max,
			"min":   min,
		}
	}

	return results, rows.Err()
}
