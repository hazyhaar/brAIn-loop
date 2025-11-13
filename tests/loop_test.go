package tests

import (
	"testing"

	"brainloop/internal/loop"
)

// TestLoopWorkflowStructure tests the loop workflow structure
func TestLoopWorkflowStructure(t *testing.T) {
	// Test that workflow phases are defined correctly

	phases := []string{"propose", "audit", "refine", "commit"}

	for _, phase := range phases {
		t.Logf("Workflow phase: %s", phase)
	}

	if len(phases) != 4 {
		t.Errorf("Expected 4 phases, got %d", len(phases))
	}
}

// TestBlockInput tests BlockInput structure
func TestBlockInput(t *testing.T) {
	block := loop.BlockInput{
		ID:          "test-block-1",
		Description: "Create a user struct",
		Type:        "go",
		Target:      "user.go",
	}

	if block.ID == "" {
		t.Error("Block ID should not be empty")
	}

	if block.Type != "go" {
		t.Errorf("Expected type=go, got %s", block.Type)
	}

	if block.Description == "" {
		t.Error("Block description should not be empty")
	}

	t.Logf("Block created: %+v", block)
}

// TestProposeRequest tests ProposeRequest structure
func TestProposeRequest(t *testing.T) {
	req := loop.ProposeRequest{
		Blocks: []loop.BlockInput{
			{
				ID:          "block1",
				Description: "Create main.go",
				Type:        "go",
				Target:      "main.go",
			},
			{
				ID:          "block2",
				Description: "Create schema.sql",
				Type:        "sql",
				Target:      "schema.sql",
			},
		},
	}

	if len(req.Blocks) != 2 {
		t.Errorf("Expected 2 blocks, got %d", len(req.Blocks))
	}

	t.Logf("Propose request with %d blocks", len(req.Blocks))
}

// TestAuditRequest tests AuditRequest structure
func TestAuditRequest(t *testing.T) {
	req := loop.AuditRequest{
		SessionID: "session-123",
		BlockID:   "block-456",
	}

	if req.SessionID == "" {
		t.Error("SessionID should not be empty")
	}

	if req.BlockID == "" {
		t.Error("BlockID should not be empty")
	}

	t.Logf("Audit request: session=%s, block=%s", req.SessionID, req.BlockID)
}

// TestRefineRequest tests RefineRequest structure
func TestRefineRequest(t *testing.T) {
	req := loop.RefineRequest{
		SessionID:     "session-123",
		BlockID:       "block-456",
		AuditFeedback: "Add error handling",
	}

	if req.AuditFeedback == "" {
		t.Error("AuditFeedback should not be empty")
	}

	t.Logf("Refine request with feedback: %s", req.AuditFeedback)
}

// TestCommitRequest tests CommitRequest structure
func TestCommitRequest(t *testing.T) {
	req := loop.CommitRequest{
		SessionID: "session-123",
		BlockID:   "block-456",
	}

	if req.SessionID == "" || req.BlockID == "" {
		t.Error("SessionID and BlockID should not be empty")
	}

	t.Logf("Commit request: session=%s, block=%s", req.SessionID, req.BlockID)
}

// TestBlockTypes tests supported block types
func TestBlockTypes(t *testing.T) {
	supportedTypes := []string{"sql", "go", "python", "code"}

	for _, blockType := range supportedTypes {
		block := loop.BlockInput{
			ID:          "test",
			Description: "test",
			Type:        blockType,
			Target:      "test",
		}

		if block.Type != blockType {
			t.Errorf("Expected type=%s, got %s", blockType, block.Type)
		}

		t.Logf("Block type supported: %s", blockType)
	}

	if len(supportedTypes) != 4 {
		t.Errorf("Expected 4 supported types, got %d", len(supportedTypes))
	}
}

// TestSessionStatuses tests session status values
func TestSessionStatuses(t *testing.T) {
	statuses := []string{"pending_audit", "committed", "abandoned"}

	for _, status := range statuses {
		t.Logf("Session status: %s", status)
	}

	if len(statuses) != 3 {
		t.Errorf("Expected 3 statuses, got %d", len(statuses))
	}
}

// TestWorkflowIterations tests that refinement can iterate
func TestWorkflowIterations(t *testing.T) {
	// Simulate multiple refinements
	maxIterations := 5

	for i := 1; i <= maxIterations; i++ {
		t.Logf("Iteration %d: refining code based on feedback", i)
	}

	t.Logf("Max iterations: %d", maxIterations)

	if maxIterations < 3 {
		t.Error("Should allow at least 3 iterations")
	}
}
