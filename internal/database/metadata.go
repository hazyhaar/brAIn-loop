package database

import (
	"database/sql"
	"fmt"
	"time"
)

// MetadataDB provides helper methods for metadata database operations
type MetadataDB struct {
	db *sql.DB
}

// NewMetadataDB creates a new metadata database helper
func NewMetadataDB(db *sql.DB) *MetadataDB {
	return &MetadataDB{db: db}
}

// GetSecret retrieves a secret by name
func (m *MetadataDB) GetSecret(secretName string) (string, error) {
	var secretValue string
	err := m.db.QueryRow(`
		SELECT secret_value FROM secrets WHERE secret_name = ?
	`, secretName).Scan(&secretValue)

	if err != nil {
		return "", fmt.Errorf("failed to get secret %s: %w", secretName, err)
	}

	return secretValue, nil
}

// SetSecret stores or updates a secret
func (m *MetadataDB) SetSecret(secretName, secretValue string) error {
	now := time.Now().Unix()

	_, err := m.db.Exec(`
		INSERT OR REPLACE INTO secrets
		(secret_name, secret_value, created_at, last_rotated)
		VALUES (?, ?, COALESCE((SELECT created_at FROM secrets WHERE secret_name = ?), ?), ?)
	`, secretName, secretValue, secretName, now, now)

	return err
}

// RecordTelemetryEvent records a telemetry event
func (m *MetadataDB) RecordTelemetryEvent(eventType, description string) error {
	_, err := m.db.Exec(`
		INSERT INTO telemetry_events (timestamp, event_type, description)
		VALUES (?, ?, ?)
	`, time.Now().Unix(), eventType, description)
	return err
}

// GetTelemetryEvents retrieves telemetry events within a time range
func (m *MetadataDB) GetTelemetryEvents(startTime, endTime int64, eventType string) ([]map[string]interface{}, error) {
	var rows *sql.Rows
	var err error

	if eventType != "" {
		rows, err = m.db.Query(`
			SELECT timestamp, event_type, description
			FROM telemetry_events
			WHERE timestamp >= ? AND timestamp <= ? AND event_type = ?
			ORDER BY timestamp DESC
		`, startTime, endTime, eventType)
	} else {
		rows, err = m.db.Query(`
			SELECT timestamp, event_type, description
			FROM telemetry_events
			WHERE timestamp >= ? AND timestamp <= ?
			ORDER BY timestamp DESC
		`, startTime, endTime)
	}

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []map[string]interface{}
	for rows.Next() {
		var timestamp int64
		var evtType string
		var description sql.NullString

		if err := rows.Scan(&timestamp, &evtType, &description); err != nil {
			return nil, err
		}

		event := map[string]interface{}{
			"timestamp":  timestamp,
			"event_type": evtType,
		}

		if description.Valid {
			event["description"] = description.String
		}

		results = append(results, event)
	}

	return results, rows.Err()
}

// CheckPoisonPill checks if a poison pill signal exists
func (m *MetadataDB) CheckPoisonPill(signalType string) (bool, error) {
	var executed int
	err := m.db.QueryRow(`
		SELECT executed FROM poisonpill WHERE signal_type = ?
	`, signalType).Scan(&executed)

	if err == sql.ErrNoRows {
		return false, nil
	}

	if err != nil {
		return false, err
	}

	return executed == 1, nil
}

// ExecutePoisonPill marks a poison pill as executed
func (m *MetadataDB) ExecutePoisonPill(signalType, result string) error {
	_, err := m.db.Exec(`
		INSERT OR REPLACE INTO poisonpill
		(signal_type, executed, executed_at, execution_result)
		VALUES (?, 1, ?, ?)
	`, signalType, time.Now().Unix(), result)
	return err
}

// CreatePoisonPill creates a new poison pill signal
func (m *MetadataDB) CreatePoisonPill(signalType string) error {
	_, err := m.db.Exec(`
		INSERT OR IGNORE INTO poisonpill (signal_type, executed)
		VALUES (?, 0)
	`, signalType)
	return err
}
