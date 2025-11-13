package patterns

import (
	"os"
	"regexp"
	"strings"
)

// DetectGoPatterns detects common patterns in Go code
func DetectGoPatterns(files []string) map[string]interface{} {
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

	// Detect naming convention
	namingConvention := detectGoNamingConvention(allContent)
	patterns["naming_convention"] = namingConvention

	// Detect top imports
	topImports := detectGoTopImports(allContent, 10)
	patterns["top_imports"] = topImports

	// Detect error handling style
	errorHandling := detectGoErrorHandling(allContent)
	patterns["error_handling"] = errorHandling

	// Detect logging style
	loggingStyle := detectGoLoggingStyle(allContent)
	patterns["logging_style"] = loggingStyle

	// Detect testing framework
	testingFramework := detectGoTestingFramework(allContent)
	if testingFramework != "" {
		patterns["testing_framework"] = testingFramework
	}

	// Detect common patterns
	usesContext := strings.Contains(allContent, "context.Context")
	patterns["uses_context"] = usesContext

	usesChannels := strings.Contains(allContent, "chan ") || strings.Contains(allContent, "<-")
	patterns["uses_channels"] = usesChannels

	usesGoroutines := strings.Contains(allContent, "go ") || strings.Contains(allContent, "go func")
	patterns["uses_goroutines"] = usesGoroutines

	// Detect struct vs interface preference
	structCount := strings.Count(allContent, "type ") - strings.Count(allContent, "type interface")
	interfaceCount := strings.Count(allContent, "type interface")
	patterns["struct_count"] = structCount
	patterns["interface_count"] = interfaceCount

	return patterns
}

// detectGoNamingConvention detects the naming convention used
func detectGoNamingConvention(content string) string {
	// Go uses mixedCase/camelCase for exported and unexported names
	// Check for snake_case vs camelCase in function names

	funcRegex := regexp.MustCompile(`func\s+([a-z]\w+)`)
	matches := funcRegex.FindAllStringSubmatch(content, -1)

	snakeCaseCount := 0
	camelCaseCount := 0

	for _, match := range matches {
		funcName := match[1]
		if strings.Contains(funcName, "_") {
			snakeCaseCount++
		} else {
			camelCaseCount++
		}
	}

	if snakeCaseCount > camelCaseCount {
		return "snake_case"
	}
	return "camelCase"
}

// detectGoTopImports detects the most frequently used imports
func detectGoTopImports(content string, topN int) []string {
	importCounts := make(map[string]int)

	// Single import
	singleRegex := regexp.MustCompile(`import\s+"([^"]+)"`)
	matches := singleRegex.FindAllStringSubmatch(content, -1)
	for _, match := range matches {
		importCounts[match[1]]++
	}

	// Multi-line import block
	blockRegex := regexp.MustCompile(`import\s*\(\s*([^)]+)\)`)
	blockMatches := blockRegex.FindAllStringSubmatch(content, -1)
	for _, blockMatch := range blockMatches {
		importBlock := blockMatch[1]
		importRegex := regexp.MustCompile(`"([^"]+)"`)
		for _, imp := range importRegex.FindAllStringSubmatch(importBlock, -1) {
			importCounts[imp[1]]++
		}
	}

	// Sort by frequency
	type importFreq struct {
		path  string
		count int
	}

	var imports []importFreq
	for path, count := range importCounts {
		imports = append(imports, importFreq{path, count})
	}

	// Simple bubble sort
	for i := 0; i < len(imports); i++ {
		for j := i + 1; j < len(imports); j++ {
			if imports[j].count > imports[i].count {
				imports[i], imports[j] = imports[j], imports[i]
			}
		}
	}

	// Extract top N
	result := make([]string, 0, topN)
	for i := 0; i < len(imports) && i < topN; i++ {
		result = append(result, imports[i].path)
	}

	return result
}

// detectGoErrorHandling detects error handling patterns
func detectGoErrorHandling(content string) map[string]interface{} {
	errorHandling := make(map[string]interface{})

	// Count different error handling patterns
	ifErrNotNil := strings.Count(content, "if err != nil")
	panicCount := strings.Count(content, "panic(")
	logFatal := strings.Count(content, "log.Fatal") + strings.Count(content, "log.Fatalf")
	returnErr := strings.Count(content, "return err") + strings.Count(content, "return fmt.Errorf")

	errorHandling["if_err_not_nil_count"] = ifErrNotNil
	errorHandling["panic_count"] = panicCount
	errorHandling["log_fatal_count"] = logFatal
	errorHandling["return_err_count"] = returnErr

	// Determine primary pattern
	primaryPattern := "return_errors"
	if panicCount > ifErrNotNil {
		primaryPattern = "panic"
	} else if logFatal > ifErrNotNil {
		primaryPattern = "log_fatal"
	}
	errorHandling["primary_pattern"] = primaryPattern

	// Check for wrapped errors
	usesErrorWrapping := strings.Contains(content, "fmt.Errorf") && strings.Contains(content, "%w")
	errorHandling["uses_error_wrapping"] = usesErrorWrapping

	return errorHandling
}

// detectGoLoggingStyle detects logging patterns
func detectGoLoggingStyle(content string) map[string]interface{} {
	loggingStyle := make(map[string]interface{})

	// Count different logging libraries/styles
	stdLog := strings.Count(content, "log.Print") + strings.Count(content, "log.Fatal")
	logrus := strings.Contains(content, "logrus") || strings.Contains(content, "WithFields")
	zap := strings.Contains(content, "go.uber.org/zap")
	zerolog := strings.Contains(content, "zerolog")

	loggingStyle["std_log_count"] = stdLog
	loggingStyle["uses_logrus"] = logrus
	loggingStyle["uses_zap"] = zap
	loggingStyle["uses_zerolog"] = zerolog

	// Determine primary logger
	primaryLogger := "std_log"
	if logrus {
		primaryLogger = "logrus"
	} else if zap {
		primaryLogger = "zap"
	} else if zerolog {
		primaryLogger = "zerolog"
	} else if stdLog == 0 {
		primaryLogger = "none"
	}
	loggingStyle["primary_logger"] = primaryLogger

	return loggingStyle
}

// detectGoTestingFramework detects testing framework used
func detectGoTestingFramework(content string) string {
	if strings.Contains(content, "github.com/stretchr/testify") {
		return "testify"
	}
	if strings.Contains(content, "ginkgo") || strings.Contains(content, "gomega") {
		return "ginkgo"
	}
	if strings.Contains(content, "func Test") {
		return "testing"
	}
	return ""
}

// DetectGoModules detects Go module patterns from go.mod
func DetectGoModules(goModPath string) (map[string]interface{}, error) {
	content, err := os.ReadFile(goModPath)
	if err != nil {
		return nil, err
	}

	patterns := make(map[string]interface{})

	// Extract module name
	moduleRegex := regexp.MustCompile(`module\s+([^\s]+)`)
	if matches := moduleRegex.FindStringSubmatch(string(content)); matches != nil {
		patterns["module_name"] = matches[1]
	}

	// Extract Go version
	goVersionRegex := regexp.MustCompile(`go\s+([\d.]+)`)
	if matches := goVersionRegex.FindStringSubmatch(string(content)); matches != nil {
		patterns["go_version"] = matches[1]
	}

	// Count dependencies
	requireCount := strings.Count(string(content), "require ")
	patterns["dependency_count"] = requireCount

	return patterns, nil
}
