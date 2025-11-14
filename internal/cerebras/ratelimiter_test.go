package cerebras

import (
	"context"
	"testing"
	"time"
)

func TestNewRateLimiter(t *testing.T) {
	rl := NewRateLimiter(60)
	defer rl.Close()

	stats := rl.GetStats()
	if stats.RequestsPerMinute != 60 {
		t.Errorf("Expected 60 rpm, got %d", stats.RequestsPerMinute)
	}

	if stats.TokensAvailable != 60 {
		t.Errorf("Expected 60 tokens initially, got %d", stats.TokensAvailable)
	}
}

func TestWait(t *testing.T) {
	rl := NewRateLimiter(5) // 5 tokens
	defer rl.Close()

	ctx := context.Background()

	// Consume all 5 tokens
	for i := 0; i < 5; i++ {
		err := rl.Wait(ctx)
		if err != nil {
			t.Fatalf("Failed to acquire token %d: %v", i, err)
		}
	}

	// 6th request should block (we'll use timeout)
	ctx6, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
	defer cancel()

	err := rl.Wait(ctx6)
	if err != context.DeadlineExceeded {
		t.Errorf("Expected DeadlineExceeded, got %v", err)
	}
}

func TestTryAcquire(t *testing.T) {
	rl := NewRateLimiter(3)
	defer rl.Close()

	// Should succeed 3 times
	for i := 0; i < 3; i++ {
		if !rl.TryAcquire() {
			t.Errorf("TryAcquire %d failed", i)
		}
	}

	// 4th should fail (no blocking)
	if rl.TryAcquire() {
		t.Error("TryAcquire should have failed (no tokens)")
	}
}

func TestRecordSuccess(t *testing.T) {
	rl := NewRateLimiter(60)
	defer rl.Close()

	// Simulate errors
	rl.RecordError()
	rl.RecordError()

	stats := rl.GetStats()
	if stats.ConsecutiveErrors != 2 {
		t.Errorf("Expected 2 consecutive errors, got %d", stats.ConsecutiveErrors)
	}

	// Record success should reset
	rl.RecordSuccess()

	stats = rl.GetStats()
	if stats.ConsecutiveErrors != 0 {
		t.Errorf("Expected 0 consecutive errors after success, got %d", stats.ConsecutiveErrors)
	}
}

func TestExponentialBackoff(t *testing.T) {
	rl := NewRateLimiter(60)
	defer rl.Close()

	// First error: 2^1 = 2 seconds backoff
	rl.RecordError()
	stats := rl.GetStats()
	if stats.InBackoff {
		expectedBackoff := 2 * time.Second
		if stats.BackoffRemaining < expectedBackoff-100*time.Millisecond ||
			stats.BackoffRemaining > expectedBackoff+100*time.Millisecond {
			t.Errorf("Expected backoff ~2s, got %s", stats.BackoffRemaining)
		}
	}

	// Second error: 2^2 = 4 seconds backoff
	rl.RecordError()
	stats = rl.GetStats()
	if stats.ConsecutiveErrors != 2 {
		t.Errorf("Expected 2 errors, got %d", stats.ConsecutiveErrors)
	}

	// Third error: 2^3 = 8 seconds backoff
	rl.RecordError()
	stats = rl.GetStats()
	if stats.ConsecutiveErrors != 3 {
		t.Errorf("Expected 3 errors, got %d", stats.ConsecutiveErrors)
	}
}

func TestBackoffMax(t *testing.T) {
	rl := NewRateLimiter(60)
	defer rl.Close()

	// Simulate many errors to trigger max backoff (300s)
	for i := 0; i < 10; i++ {
		rl.RecordError()
	}

	backoff := rl.GetBackoffDuration()
	if backoff != 300*time.Second {
		t.Errorf("Expected max backoff 300s, got %s", backoff)
	}
}

func TestIsInBackoff(t *testing.T) {
	rl := NewRateLimiter(60)
	defer rl.Close()

	// No backoff initially
	if rl.isInBackoff() {
		t.Error("Should not be in backoff initially")
	}

	// Record error triggers backoff
	rl.RecordError()
	if !rl.isInBackoff() {
		t.Error("Should be in backoff after error")
	}

	// Wait for backoff to expire
	time.Sleep(2500 * time.Millisecond) // 2s backoff + margin

	if rl.isInBackoff() {
		t.Error("Should not be in backoff after expiry")
	}
}

func TestWaitDuringBackoff(t *testing.T) {
	rl := NewRateLimiter(60)
	defer rl.Close()

	// Trigger backoff
	rl.RecordError()

	ctx := context.Background()
	err := rl.Wait(ctx)

	if err == nil {
		t.Error("Wait should fail during backoff")
	}

	if !rl.isInBackoff() {
		t.Error("Should still be in backoff")
	}
}

func TestResetBackoff(t *testing.T) {
	rl := NewRateLimiter(60)
	defer rl.Close()

	// Trigger backoff
	rl.RecordError()
	rl.RecordError()

	stats := rl.GetStats()
	if !stats.InBackoff {
		t.Error("Should be in backoff")
	}

	// Manual reset
	rl.ResetBackoff()

	stats = rl.GetStats()
	if stats.InBackoff {
		t.Error("Should not be in backoff after reset")
	}

	if stats.ConsecutiveErrors != 0 {
		t.Errorf("Expected 0 errors after reset, got %d", stats.ConsecutiveErrors)
	}
}

func TestSetRate(t *testing.T) {
	rl := NewRateLimiter(10)
	defer rl.Close()

	// Consume 5 tokens
	for i := 0; i < 5; i++ {
		rl.TryAcquire()
	}

	// Change rate to 20
	err := rl.SetRate(20)
	if err != nil {
		t.Fatalf("SetRate failed: %v", err)
	}

	stats := rl.GetStats()
	if stats.RequestsPerMinute != 20 {
		t.Errorf("Expected 20 rpm, got %d", stats.RequestsPerMinute)
	}

	// Should have tokens available (transferred + new)
	if stats.TokensAvailable < 5 {
		t.Errorf("Expected at least 5 tokens, got %d", stats.TokensAvailable)
	}
}

func TestSetRateInvalid(t *testing.T) {
	rl := NewRateLimiter(60)
	defer rl.Close()

	err := rl.SetRate(0)
	if err == nil {
		t.Error("SetRate(0) should fail")
	}

	err = rl.SetRate(-10)
	if err == nil {
		t.Error("SetRate(-10) should fail")
	}
}

func TestRetryWithBackoff(t *testing.T) {
	rl := NewRateLimiter(60)
	defer rl.Close()

	ctx := context.Background()
	config := RetryConfig{
		MaxRetries:        3,
		InitialBackoff:    10 * time.Millisecond,
		MaxBackoff:        100 * time.Millisecond,
		BackoffMultiplier: 2.0,
	}

	// Test successful execution on first try
	callCount := 0
	fn := func() error {
		callCount++
		return nil
	}

	err := RetryWithBackoff(ctx, rl, config, fn)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if callCount != 1 {
		t.Errorf("Expected 1 call, got %d", callCount)
	}
}

func TestRetryWithBackoffFailures(t *testing.T) {
	rl := NewRateLimiter(60)
	defer rl.Close()

	ctx := context.Background()
	config := RetryConfig{
		MaxRetries:        2,
		InitialBackoff:    5 * time.Millisecond,
		MaxBackoff:        50 * time.Millisecond,
		BackoffMultiplier: 2.0,
	}

	// Fail first 2 times, succeed on 3rd
	callCount := 0
	fn := func() error {
		callCount++
		if callCount < 3 {
			return &TemporaryError{Msg: "temporary failure"}
		}
		return nil
	}

	err := RetryWithBackoff(ctx, rl, config, fn)
	if err != nil {
		t.Errorf("Should succeed after retries: %v", err)
	}

	if callCount != 3 {
		t.Errorf("Expected 3 calls, got %d", callCount)
	}
}

func TestRetryWithBackoffMaxRetries(t *testing.T) {
	rl := NewRateLimiter(60)
	defer rl.Close()

	ctx := context.Background()
	config := RetryConfig{
		MaxRetries:        2,
		InitialBackoff:    5 * time.Millisecond,
		MaxBackoff:        50 * time.Millisecond,
		BackoffMultiplier: 2.0,
	}

	// Always fail
	callCount := 0
	fn := func() error {
		callCount++
		return &TemporaryError{Msg: "always fails"}
	}

	err := RetryWithBackoff(ctx, rl, config, fn)
	if err == nil {
		t.Error("Should fail after max retries")
	}

	// Should be called: initial + 2 retries = 3 times
	if callCount != 3 {
		t.Errorf("Expected 3 calls (1 + 2 retries), got %d", callCount)
	}
}

func TestRetryWithBackoffContextCancellation(t *testing.T) {
	rl := NewRateLimiter(60)
	defer rl.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	config := RetryConfig{
		MaxRetries:        10,
		InitialBackoff:    100 * time.Millisecond,
		MaxBackoff:        1 * time.Second,
		BackoffMultiplier: 2.0,
	}

	fn := func() error {
		return &TemporaryError{Msg: "always fails"}
	}

	err := RetryWithBackoff(ctx, rl, config, fn)
	if err != context.DeadlineExceeded {
		t.Errorf("Expected DeadlineExceeded, got %v", err)
	}
}

func TestConcurrentWait(t *testing.T) {
	rl := NewRateLimiter(10)
	defer rl.Close()

	ctx := context.Background()
	successCount := 0
	done := make(chan bool)

	// Spawn 20 goroutines trying to acquire tokens
	for i := 0; i < 20; i++ {
		go func() {
			err := rl.Wait(ctx)
			if err == nil {
				successCount++
			}
			done <- true
		}()
	}

	// Wait for all goroutines (with timeout)
	timeout := time.After(2 * time.Second)
	for i := 0; i < 20; i++ {
		select {
		case <-done:
		case <-timeout:
			t.Fatal("Timeout waiting for goroutines")
		}
	}

	// Only 10 should succeed (initial tokens)
	if successCount != 10 {
		t.Errorf("Expected 10 successful acquires, got %d", successCount)
	}
}

// TemporaryError is a test error type
type TemporaryError struct {
	Msg string
}

func (e *TemporaryError) Error() string {
	return e.Msg
}

func (e *TemporaryError) Temporary() bool {
	return true
}

func TestDefaultRetryConfig(t *testing.T) {
	config := DefaultRetryConfig()

	if config.MaxRetries != 4 {
		t.Errorf("Expected max retries 4, got %d", config.MaxRetries)
	}

	if config.InitialBackoff != 2*time.Second {
		t.Errorf("Expected initial backoff 2s, got %s", config.InitialBackoff)
	}

	if config.MaxBackoff != 60*time.Second {
		t.Errorf("Expected max backoff 60s, got %s", config.MaxBackoff)
	}

	if config.BackoffMultiplier != 2.0 {
		t.Errorf("Expected multiplier 2.0, got %f", config.BackoffMultiplier)
	}
}
