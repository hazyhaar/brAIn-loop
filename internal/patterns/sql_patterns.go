package patterns

import (
	"os"
	"regexp"
	"strings"
)

// DetectSQLPatterns detects common patterns in SQL code
func DetectSQLPatterns(files []string) map[string]interface{} {
	patterns := make(map[string]interface{})

	// Collect all file contents
	var allContent string
	for _, file := range files {
		content, err := os.ReadFile(file)
		if err != nil {
			continue
		}
		allContent += string(content) + "\n"
	}

	// Convert to uppercase for case-insensitive matching
	upperContent := strings.ToUpper(allContent)

	// Detect pragmas used
	pragmas := detectSQLPragmas(allContent)
	patterns["pragmas"] = pragmas

	// Detect table naming convention
	tableNaming := detectSQLTableNaming(allContent)
	patterns["table_naming"] = tableNaming

	// Detect CREATE TABLE style
	createTableStyle := detectSQLCreateTableStyle(allContent)
	patterns["create_table_style"] = createTableStyle

	// Detect constraint usage
	constraints := detectSQLConstraints(allContent)
	patterns["constraints"] = constraints

	// Detect index usage
	indexUsage := detectSQLIndexUsage(allContent)
	patterns["index_usage"] = indexUsage

	// Detect transaction usage
	transactionUsage := detectSQLTransactionUsage(upperContent)
	patterns["transaction_usage"] = transactionUsage

	// Count tables and indexes
	tableCount := strings.Count(upperContent, "CREATE TABLE")
	indexCount := strings.Count(upperContent, "CREATE INDEX")
	patterns["table_count"] = tableCount
	patterns["index_count"] = indexCount

	return patterns
}

// detectSQLPragmas detects PRAGMA statements used
func detectSQLPragmas(content string) []string {
	var pragmas []string
	seen := make(map[string]bool)

	pragmaRegex := regexp.MustCompile(`(?i)PRAGMA\s+([a-z_]+)\s*=?\s*([^;]+)`)
	matches := pragmaRegex.FindAllStringSubmatch(content, -1)

	for _, match := range matches {
		pragma := strings.TrimSpace(match[1] + " = " + strings.TrimSpace(match[2]))
		if !seen[pragma] {
			seen[pragma] = true
			pragmas = append(pragmas, pragma)
		}
	}

	return pragmas
}

// detectSQLTableNaming detects table naming convention
func detectSQLTableNaming(content string) map[string]interface{} {
	naming := make(map[string]interface{})

	// Extract table names
	tableRegex := regexp.MustCompile(`(?i)CREATE\s+TABLE\s+(?:IF\s+NOT\s+EXISTS\s+)?([a-z_][a-z0-9_]*)`)
	matches := tableRegex.FindAllStringSubmatch(content, -1)

	var tableNames []string
	for _, match := range matches {
		tableNames = append(tableNames, match[1])
	}

	naming["table_names"] = tableNames

	// Detect naming patterns
	singularCount := 0
	pluralCount := 0
	snakeCaseCount := 0

	for _, name := range tableNames {
		// Check for snake_case
		if strings.Contains(name, "_") {
			snakeCaseCount++
		}

		// Simple plural detection (ends with 's')
		if strings.HasSuffix(name, "s") {
			pluralCount++
		} else {
			singularCount++
		}
	}

	if snakeCaseCount > 0 {
		naming["case_style"] = "snake_case"
	} else {
		naming["case_style"] = "lowercase"
	}

	if pluralCount > singularCount {
		naming["number_style"] = "plural"
	} else {
		naming["number_style"] = "singular"
	}

	return naming
}

// detectSQLCreateTableStyle detects CREATE TABLE patterns
func detectSQLCreateTableStyle(content string) map[string]interface{} {
	style := make(map[string]interface{})

	upperContent := strings.ToUpper(content)

	// Count IF NOT EXISTS usage
	ifNotExists := strings.Count(upperContent, "IF NOT EXISTS")
	totalCreateTable := strings.Count(upperContent, "CREATE TABLE")

	style["uses_if_not_exists"] = ifNotExists > 0
	style["if_not_exists_ratio"] = float64(ifNotExists) / float64(totalCreateTable)

	// Check for inline constraints
	usesInlineConstraints := strings.Contains(upperContent, "PRIMARY KEY") ||
		strings.Contains(upperContent, "NOT NULL")
	style["uses_inline_constraints"] = usesInlineConstraints

	// Check for foreign keys
	usesForeignKeys := strings.Contains(upperContent, "FOREIGN KEY")
	style["uses_foreign_keys"] = usesForeignKeys

	// Check for default values
	usesDefaults := strings.Contains(upperContent, "DEFAULT ")
	style["uses_defaults"] = usesDefaults

	return style
}

// detectSQLConstraints detects constraint usage patterns
func detectSQLConstraints(content string) map[string]interface{} {
	constraints := make(map[string]interface{})

	upperContent := strings.ToUpper(content)

	// Count constraint types
	primaryKeyCount := strings.Count(upperContent, "PRIMARY KEY")
	foreignKeyCount := strings.Count(upperContent, "FOREIGN KEY")
	uniqueCount := strings.Count(upperContent, "UNIQUE")
	notNullCount := strings.Count(upperContent, "NOT NULL")
	checkCount := strings.Count(upperContent, "CHECK ")

	constraints["primary_key_count"] = primaryKeyCount
	constraints["foreign_key_count"] = foreignKeyCount
	constraints["unique_count"] = uniqueCount
	constraints["not_null_count"] = notNullCount
	constraints["check_count"] = checkCount

	// Determine constraint preference
	totalConstraints := primaryKeyCount + foreignKeyCount + uniqueCount + checkCount
	constraints["total_constraints"] = totalConstraints

	return constraints
}

// detectSQLIndexUsage detects index usage patterns
func detectSQLIndexUsage(content string) map[string]interface{} {
	indexUsage := make(map[string]interface{})

	upperContent := strings.ToUpper(content)

	// Count index types
	standardIndex := strings.Count(upperContent, "CREATE INDEX")
	uniqueIndex := strings.Count(upperContent, "CREATE UNIQUE INDEX")

	indexUsage["standard_index_count"] = standardIndex
	indexUsage["unique_index_count"] = uniqueIndex
	indexUsage["total_index_count"] = standardIndex + uniqueIndex

	// Check for IF NOT EXISTS on indexes
	usesIndexIfNotExists := strings.Contains(content, "CREATE INDEX IF NOT EXISTS") ||
		strings.Contains(content, "CREATE UNIQUE INDEX IF NOT EXISTS")
	indexUsage["uses_if_not_exists"] = usesIndexIfNotExists

	// Detect index naming pattern
	indexRegex := regexp.MustCompile(`(?i)CREATE\s+(?:UNIQUE\s+)?INDEX\s+(?:IF\s+NOT\s+EXISTS\s+)?([a-z_][a-z0-9_]*)`)
	matches := indexRegex.FindAllStringSubmatch(content, -1)

	var indexNames []string
	for _, match := range matches {
		indexNames = append(indexNames, match[1])
	}

	// Check if index names start with "idx_"
	prefixCount := 0
	for _, name := range indexNames {
		if strings.HasPrefix(name, "idx_") {
			prefixCount++
		}
	}

	if len(indexNames) > 0 && prefixCount > len(indexNames)/2 {
		indexUsage["naming_convention"] = "idx_ prefix"
	} else {
		indexUsage["naming_convention"] = "custom"
	}

	return indexUsage
}

// detectSQLTransactionUsage detects transaction patterns
func detectSQLTransactionUsage(upperContent string) map[string]interface{} {
	transactionUsage := make(map[string]interface{})

	beginCount := strings.Count(upperContent, "BEGIN TRANSACTION") + strings.Count(upperContent, "BEGIN")
	commitCount := strings.Count(upperContent, "COMMIT")
	rollbackCount := strings.Count(upperContent, "ROLLBACK")

	transactionUsage["begin_count"] = beginCount
	transactionUsage["commit_count"] = commitCount
	transactionUsage["rollback_count"] = rollbackCount

	usesTransactions := beginCount > 0 || commitCount > 0
	transactionUsage["uses_transactions"] = usesTransactions

	return transactionUsage
}

// DetectSQLiteSpecificPatterns detects SQLite-specific patterns
func DetectSQLiteSpecificPatterns(content string) map[string]interface{} {
	patterns := make(map[string]interface{})

	upperContent := strings.ToUpper(content)

	// Check for SQLite-specific features
	usesAutoincrement := strings.Contains(upperContent, "AUTOINCREMENT")
	patterns["uses_autoincrement"] = usesAutoincrement

	usesWithoutRowid := strings.Contains(upperContent, "WITHOUT ROWID")
	patterns["uses_without_rowid"] = usesWithoutRowid

	usesStrict := strings.Contains(upperContent, "STRICT")
	patterns["uses_strict"] = usesStrict

	// Check data types
	usesText := strings.Contains(upperContent, "TEXT")
	usesInteger := strings.Contains(upperContent, "INTEGER")
	usesReal := strings.Contains(upperContent, "REAL")
	usesBlob := strings.Contains(upperContent, "BLOB")

	patterns["uses_text"] = usesText
	patterns["uses_integer"] = usesInteger
	patterns["uses_real"] = usesReal
	patterns["uses_blob"] = usesBlob

	return patterns
}
