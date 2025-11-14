package mcp

import (
	"bufio"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"

	"brainloop/internal/cerebras"
	"brainloop/internal/database"
	"brainloop/internal/loop"
	"brainloop/internal/patterns"
	"brainloop/internal/readers"
)

// Server represents an MCP server
type Server struct {
	lifecycleDB     *sql.DB
	outputDB        *sql.DB
	metadataDB      *sql.DB
	cerebrasClient  *cerebras.Client
	loopManager     *loop.Manager
	readersHub      *readers.Hub
	patternExtractor *patterns.Extractor
	bashHandler     *BashHandler
	ctx             context.Context
	cancel          context.CancelFunc
}

// NewServer creates a new MCP server
func NewServer(lifecycleDB, outputDB, metadataDB *sql.DB) (*Server, error) {
	// Get Cerebras API key from metadata DB
	metaDB := database.NewMetadataDB(metadataDB)
	apiKey, err := metaDB.GetSecret("CEREBRAS_API_KEY")
	if err != nil {
		return nil, fmt.Errorf("failed to get Cerebras API key: %w", err)
	}

	// Initialize Cerebras client
	cerebrasClient := cerebras.NewClient(apiKey)

	// Initialize loop manager
	loopManager := loop.NewManager(lifecycleDB, outputDB, cerebrasClient)

	// Initialize readers hub
	readersHub := readers.NewHub(lifecycleDB, outputDB, cerebrasClient)

	// Initialize pattern extractor
	patternExtractor := patterns.NewExtractor(lifecycleDB)

	// Initialize bash handler
	bashHandler, err := NewBashHandler("command_security.db")
	if err != nil {
		return nil, fmt.Errorf("failed to initialize bash handler: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &Server{
		lifecycleDB:      lifecycleDB,
		outputDB:         outputDB,
		metadataDB:       metadataDB,
		cerebrasClient:   cerebrasClient,
		loopManager:      loopManager,
		readersHub:       readersHub,
		patternExtractor: patternExtractor,
		bashHandler:      bashHandler,
		ctx:              ctx,
		cancel:           cancel,
	}, nil
}

// JSONRPCRequest represents a JSON-RPC 2.0 request
type JSONRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// JSONRPCResponse represents a JSON-RPC 2.0 response
type JSONRPCResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id"`
	Result  interface{} `json:"result,omitempty"`
	Error   *RPCError   `json:"error,omitempty"`
}

// RPCError represents a JSON-RPC error
type RPCError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// Serve starts the MCP server on stdin/stdout
func (s *Server) Serve(stdin io.Reader, stdout io.Writer) error {
	scanner := bufio.NewScanner(stdin)

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		// Parse JSON-RPC request
		var req JSONRPCRequest
		if err := json.Unmarshal([]byte(line), &req); err != nil {
			s.sendError(stdout, nil, -32700, "Parse error", err.Error())
			continue
		}

		// Handle request
		response := s.handleRequest(&req)

		// Send response
		responseJSON, err := json.Marshal(response)
		if err != nil {
			log.Printf("Failed to marshal response: %v", err)
			continue
		}

		fmt.Fprintln(stdout, string(responseJSON))
	}

	return scanner.Err()
}

// handleRequest routes requests to appropriate handlers
func (s *Server) handleRequest(req *JSONRPCRequest) *JSONRPCResponse {
	switch req.Method {
	case "initialize":
		return s.handleInitialize(req)
	case "tools/list":
		return s.handleToolsList(req)
	case "tools/call":
		return s.handleToolCall(req)
	default:
		return &JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &RPCError{
				Code:    -32601,
				Message: "Method not found",
			},
		}
	}
}

// handleInitialize handles initialization request
func (s *Server) handleInitialize(req *JSONRPCRequest) *JSONRPCResponse {
	return &JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result: map[string]interface{}{
			"protocolVersion": "2024-11-05",
			"capabilities": map[string]interface{}{
				"tools": map[string]interface{}{},
			},
			"serverInfo": map[string]interface{}{
				"name":    "brainloop",
				"version": "1.0.0",
			},
		},
	}
}

// handleToolsList handles tools/list request
func (s *Server) handleToolsList(req *JSONRPCRequest) *JSONRPCResponse {
	// Progressive disclosure: expose only 1 tool
	tools := []map[string]interface{}{
		{
			"name":        "brainloop",
			"description": "Cerebras-powered code generation and intelligent reading with progressive disclosure",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"action": map[string]interface{}{
						"type": "string",
						"enum": []string{
							"generate_file", "generate_sql", "explore", "loop",
							"read_sqlite", "read_markdown", "read_code", "read_config",
							"list_actions", "get_schema", "get_stats",
						},
						"description": "Action to perform. Use 'list_actions' to see all available actions with descriptions.",
					},
					"params": map[string]interface{}{
						"type":        "object",
						"description": "Action-specific parameters. Use 'get_schema' action to see schema for specific action.",
					},
				},
				"required": []string{"action", "params"},
			},
		},
	}

	return &JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result: map[string]interface{}{
			"tools": tools,
		},
	}
}

// handleToolCall handles tools/call request
func (s *Server) handleToolCall(req *JSONRPCRequest) *JSONRPCResponse {
	var params struct {
		Name      string                 `json:"name"`
		Arguments map[string]interface{} `json:"arguments"`
	}

	if err := json.Unmarshal(req.Params, &params); err != nil {
		return s.errorResponse(req.ID, -32602, "Invalid params", err.Error())
	}

	// Ensure tool name is "brainloop"
	if params.Name != "brainloop" {
		return s.errorResponse(req.ID, -32602, "Unknown tool", params.Name)
	}

	// Extract action and params
	action, ok := params.Arguments["action"].(string)
	if !ok {
		return s.errorResponse(req.ID, -32602, "Missing action parameter", nil)
	}

	actionParams, ok := params.Arguments["params"].(map[string]interface{})
	if !ok {
		actionParams = make(map[string]interface{})
	}

	// Dispatch to tool handler
	result, err := s.dispatchAction(action, actionParams)
	if err != nil {
		return s.errorResponse(req.ID, -32000, "Action failed", err.Error())
	}

	return &JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result: map[string]interface{}{
			"content": []map[string]interface{}{
				{
					"type": "text",
					"text": fmt.Sprintf("%v", result),
				},
			},
		},
	}
}

// Shutdown gracefully shuts down the server
func (s *Server) Shutdown(ctx context.Context) error {
	s.cancel()
	return nil
}

// sendError sends an error response
func (s *Server) sendError(stdout io.Writer, id interface{}, code int, message, data string) {
	response := &JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error: &RPCError{
			Code:    code,
			Message: message,
			Data:    data,
		},
	}

	responseJSON, _ := json.Marshal(response)
	fmt.Fprintln(stdout, string(responseJSON))
}

// errorResponse creates an error response
func (s *Server) errorResponse(id interface{}, code int, message string, data interface{}) *JSONRPCResponse {
	return &JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error: &RPCError{
			Code:    code,
			Message: message,
			Data:    data,
		},
	}
}
