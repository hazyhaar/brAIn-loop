package readers

import (
	"database/sql"
	"encoding/json"
	"fmt"

	_ "modernc.org/sqlite"
)

// ReadSQLite reads and analyzes a SQLite database
func (h *Hub) ReadSQLite(params map[string]interface{}) (string, error) {
	// Extract parameters
	dbPath, ok := params["db_path"].(string)
	if !ok {
		return "", fmt.Errorf("missing or invalid db_path parameter")
	}

	maxSampleRows := 5
	if rows, ok := params["max_sample_rows"].(float64); ok {
		maxSampleRows = int(rows)
	}

	// Compute hash for caching
	hash, err := h.computeHash(dbPath)
	if err != nil {
		return "", err
	}

	// Check cache
	if digest, found := h.checkCache(hash); found {
		h.outputDB.RecordMetric("reader_cache_hit", 1.0)
		return digest, nil
	}

	h.outputDB.RecordMetric("reader_cache_miss", 1.0)

	// Open database
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return "", fmt.Errorf("failed to open database: %w", err)
	}
	defer db.Close()

	// Analyze database
	analysis, err := h.analyzeSQLiteDB(db, maxSampleRows)
	if err != nil {
		return "", fmt.Errorf("failed to analyze database: %w", err)
	}

	// Format analysis as string for Cerebras
	analysisJSON, err := json.MarshalIndent(analysis, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal analysis: %w", err)
	}

	// Generate digest using Cerebras
	digest, err := h.generateDigest("sqlite", string(analysisJSON))
	if err != nil {
		return "", err
	}

	// Save to cache
	if err := h.saveCache(hash, "sqlite", dbPath, digest); err != nil {
		// Log but don't fail
		fmt.Printf("Warning: failed to save cache: %v\n", err)
	}

	// Publish to output
	if err := h.publishDigest(hash, "sqlite", dbPath, digest); err != nil {
		// Log but don't fail
		fmt.Printf("Warning: failed to publish digest: %v\n", err)
	}

	return digest, nil
}

// analyzeSQLiteDB performs comprehensive analysis of a SQLite database
func (h *Hub) analyzeSQLiteDB(db *sql.DB, maxSampleRows int) (map[string]interface{}, error) {
	analysis := make(map[string]interface{})

	// Get pragmas
	pragmas, err := h.getSQLitePragmas(db)
	if err != nil {
		return nil, fmt.Errorf("failed to get pragmas: %w", err)
	}
	analysis["pragmas"] = pragmas

	// Get tables
	tables, err := h.getSQLiteTables(db, maxSampleRows)
	if err != nil {
		return nil, fmt.Errorf("failed to get tables: %w", err)
	}
	analysis["tables"] = tables
	analysis["table_count"] = len(tables)

	// Get schemas (DDL)
	schemas, err := h.getSQLiteSchemas(db)
	if err != nil {
		return nil, fmt.Errorf("failed to get schemas: %w", err)
	}
	analysis["schemas"] = schemas

	// Get indexes
	indexes, err := h.getSQLiteIndexes(db)
	if err != nil {
		return nil, fmt.Errorf("failed to get indexes: %w", err)
	}
	analysis["indexes"] = indexes

	// Database size
	var pageCount, pageSize int
	db.QueryRow("PRAGMA page_count").Scan(&pageCount)
	db.QueryRow("PRAGMA page_size").Scan(&pageSize)
	analysis["database_size_bytes"] = pageCount * pageSize

	return analysis, nil
}

// getSQLitePragmas retrieves important pragma settings
func (h *Hub) getSQLitePragmas(db *sql.DB) (map[string]interface{}, error) {
	pragmas := make(map[string]interface{})

	pragmaQueries := []string{
		"PRAGMA journal_mode",
		"PRAGMA synchronous",
		"PRAGMA foreign_keys",
		"PRAGMA cache_size",
		"PRAGMA page_size",
	}

	for _, query := range pragmaQueries {
		var value string
		if err := db.QueryRow(query).Scan(&value); err != nil {
			continue
		}
		pragmas[query] = value
	}

	return pragmas, nil
}

// getSQLiteTables retrieves all tables with metadata
func (h *Hub) getSQLiteTables(db *sql.DB, maxSampleRows int) ([]map[string]interface{}, error) {
	// Get table names
	rows, err := db.Query("SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%'")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tableNames []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		tableNames = append(tableNames, name)
	}

	// Analyze each table
	var tables []map[string]interface{}
	for _, tableName := range tableNames {
		tableInfo := make(map[string]interface{})
		tableInfo["name"] = tableName

		// Get columns
		columns, err := h.getTableColumns(db, tableName)
		if err != nil {
			return nil, fmt.Errorf("failed to get columns for table %s: %w", tableName, err)
		}
		tableInfo["columns"] = columns

		// Get row count
		var rowCount int
		db.QueryRow(fmt.Sprintf("SELECT COUNT(*) FROM \"%s\"", tableName)).Scan(&rowCount)
		tableInfo["row_count"] = rowCount

		// Get sample rows
		if rowCount > 0 && maxSampleRows > 0 {
			samples, err := h.getSampleRows(db, tableName, maxSampleRows)
			if err == nil {
				tableInfo["sample_data"] = samples
			}
		}

		tables = append(tables, tableInfo)
	}

	return tables, nil
}

// getTableColumns retrieves column information for a table
func (h *Hub) getTableColumns(db *sql.DB, tableName string) ([]map[string]interface{}, error) {
	rows, err := db.Query(fmt.Sprintf("PRAGMA table_info(\"%s\")", tableName))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var columns []map[string]interface{}
	for rows.Next() {
		var cid int
		var name, colType string
		var notNull, pk int
		var dfltValue sql.NullString

		if err := rows.Scan(&cid, &name, &colType, &notNull, &dfltValue, &pk); err != nil {
			return nil, err
		}

		column := map[string]interface{}{
			"name":     name,
			"type":     colType,
			"not_null": notNull == 1,
			"pk":       pk == 1,
		}

		if dfltValue.Valid {
			column["default"] = dfltValue.String
		}

		columns = append(columns, column)
	}

	return columns, nil
}

// getSampleRows retrieves sample rows from a table
func (h *Hub) getSampleRows(db *sql.DB, tableName string, limit int) ([]map[string]interface{}, error) {
	query := fmt.Sprintf("SELECT * FROM \"%s\" LIMIT %d", tableName, limit)
	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	var samples []map[string]interface{}
	for rows.Next() {
		// Create a slice of interface{} to hold values
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			return nil, err
		}

		// Create map for this row
		row := make(map[string]interface{})
		for i, col := range columns {
			val := values[i]
			// Convert byte slices to strings for JSON
			if b, ok := val.([]byte); ok {
				row[col] = string(b)
			} else {
				row[col] = val
			}
		}

		samples = append(samples, row)
	}

	return samples, nil
}

// getSQLiteSchemas retrieves DDL statements
func (h *Hub) getSQLiteSchemas(db *sql.DB) ([]string, error) {
	rows, err := db.Query("SELECT sql FROM sqlite_master WHERE type IN ('table', 'index') AND sql IS NOT NULL")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var schemas []string
	for rows.Next() {
		var sql string
		if err := rows.Scan(&sql); err != nil {
			return nil, err
		}
		schemas = append(schemas, sql)
	}

	return schemas, nil
}

// getSQLiteIndexes retrieves index information
func (h *Hub) getSQLiteIndexes(db *sql.DB) ([]map[string]interface{}, error) {
	rows, err := db.Query("SELECT name, tbl_name, sql FROM sqlite_master WHERE type='index' AND sql IS NOT NULL")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var indexes []map[string]interface{}
	for rows.Next() {
		var name, tableName string
		var sqlStmt sql.NullString

		if err := rows.Scan(&name, &tableName, &sqlStmt); err != nil {
			return nil, err
		}

		index := map[string]interface{}{
			"name":  name,
			"table": tableName,
		}

		if sqlStmt.Valid {
			index["sql"] = sqlStmt.String
		}

		indexes = append(indexes, index)
	}

	return indexes, nil
}
