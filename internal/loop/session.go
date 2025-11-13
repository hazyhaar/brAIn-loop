package loop

// Session represents a cerebras_loop session
type Session struct {
	SessionID   string  `json:"session_id"`
	Status      string  `json:"status"` // 'pending_audit' | 'committed' | 'abandoned'
	Blocks      []Block `json:"blocks"`
	CreatedAt   int64   `json:"created_at"`
	CompletedAt int64   `json:"completed_at,omitempty"`
}

// Block represents a code block in a session
type Block struct {
	BlockID        string        `json:"block_id"`
	SessionID      string        `json:"session_id"`
	Description    string        `json:"description"`
	Type           string        `json:"type"`   // 'sql' | 'go' | 'python' | 'code'
	Target         string        `json:"target"` // file_path or db_path
	Code           string        `json:"code,omitempty"`
	Iterations     int           `json:"iterations"`
	Status         string        `json:"status"` // 'pending' | 'committed'
	GeneratedAt    int64         `json:"generated_at"`
	LastRefinedAt  int64         `json:"last_refined_at,omitempty"`
	CommittedAt    int64         `json:"committed_at,omitempty"`
	Refinements    []Refinement  `json:"refinements,omitempty"`
}

// Refinement represents an audit refinement for a block
type Refinement struct {
	RefinementID string  `json:"refinement_id"`
	BlockID      string  `json:"block_id"`
	Feedback     string  `json:"feedback"`
	Temperature  float64 `json:"temperature"`
	RefinedCode  string  `json:"refined_code"`
	CreatedAt    int64   `json:"created_at"`
}

// BlockInput represents input for creating a block
type BlockInput struct {
	ID          string `json:"id"`
	Description string `json:"description"`
	Type        string `json:"type"`
	Target      string `json:"target"`
}

// ProposeRequest represents a request to propose a session
type ProposeRequest struct {
	Blocks []BlockInput `json:"blocks"`
}

// AuditRequest represents a request to audit a block
type AuditRequest struct {
	SessionID string `json:"session_id"`
	BlockID   string `json:"block_id"`
}

// RefineRequest represents a request to refine a block
type RefineRequest struct {
	SessionID     string `json:"session_id"`
	BlockID       string `json:"block_id"`
	AuditFeedback string `json:"audit_feedback"`
}

// CommitRequest represents a request to commit a block
type CommitRequest struct {
	SessionID string `json:"session_id"`
	BlockID   string `json:"block_id"`
}

// ProposeResponse represents the response from a propose operation
type ProposeResponse struct {
	SessionID string  `json:"session_id"`
	Blocks    []Block `json:"blocks"`
}

// AuditResponse represents the response from an audit operation
type AuditResponse struct {
	Block Block `json:"block"`
}

// RefineResponse represents the response from a refine operation
type RefineResponse struct {
	Block       Block  `json:"block"`
	RefinedCode string `json:"refined_code"`
	Iterations  int    `json:"iterations"`
}

// CommitResponse represents the response from a commit operation
type CommitResponse struct {
	Block       Block  `json:"block"`
	Success     bool   `json:"success"`
	Message     string `json:"message"`
	OutputPath  string `json:"output_path,omitempty"`
}
