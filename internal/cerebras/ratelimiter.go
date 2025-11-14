package cerebras

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// RateLimiter implements token bucket algorithm with exponential backoff
type RateLimiter struct {
	requestsPerMinute int
	tokens            chan struct{}
	lastRefill        time.Time
	mu                sync.Mutex

	// Backoff state
	consecutiveErrors int
	lastErrorTime     time.Time
	backoffDuration   time.Duration
}

// NewRateLimiter creates a new rate limiter
// rpm: requests per minute (default: 60 for Cerebras free tier)
func NewRateLimiter(rpm int) *RateLimiter {
	if rpm <= 0 {
		rpm = 60 // Default for Cerebras free tier
	}

	rl := &RateLimiter{
		requestsPerMinute: rpm,
		tokens:            make(chan struct{}, rpm),
		lastRefill:        time.Now(),
		backoffDuration:   0,
	}

	// Initial token fill
	for i := 0; i < rpm; i++ {
		rl.tokens <- struct{}{}
	}

	// Start refill goroutine
	go rl.refillLoop()

	return rl
}

// Wait waits for a token to become available
// Returns error if context is cancelled
func (rl *RateLimiter) Wait(ctx context.Context) error {
	// Check backoff
	if rl.isInBackoff() {
		backoffRemaining := rl.getBackoffRemaining()
		return fmt.Errorf("rate limited: backoff active for %s", backoffRemaining)
	}

	select {
	case <-rl.tokens:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// TryAcquire attempts to acquire a token without blocking
// Returns true if token acquired, false otherwise
func (rl *RateLimiter) TryAcquire() bool {
	if rl.isInBackoff() {
		return false
	}

	select {
	case <-rl.tokens:
		return true
	default:
		return false
	}
}

// RecordSuccess records a successful API call
// Resets exponential backoff
func (rl *RateLimiter) RecordSuccess() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	rl.consecutiveErrors = 0
	rl.backoffDuration = 0
}

// RecordError records a failed API call
// Triggers exponential backoff
func (rl *RateLimiter) RecordError() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	rl.consecutiveErrors++
	rl.lastErrorTime = time.Now()

	// Exponential backoff: 2^n seconds, max 300s (5 minutes)
	backoff := time.Duration(1<<uint(rl.consecutiveErrors)) * time.Second
	if backoff > 300*time.Second {
		backoff = 300 * time.Second
	}

	rl.backoffDuration = backoff
}

// GetBackoffDuration returns current backoff duration
func (rl *RateLimiter) GetBackoffDuration() time.Duration {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	return rl.backoffDuration
}

// isInBackoff checks if currently in backoff period
func (rl *RateLimiter) isInBackoff() bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	if rl.backoffDuration == 0 {
		return false
	}

	elapsed := time.Since(rl.lastErrorTime)
	return elapsed < rl.backoffDuration
}

// getBackoffRemaining returns remaining backoff time
func (rl *RateLimiter) getBackoffRemaining() time.Duration {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	if rl.backoffDuration == 0 {
		return 0
	}

	elapsed := time.Since(rl.lastErrorTime)
	remaining := rl.backoffDuration - elapsed

	if remaining < 0 {
		return 0
	}

	return remaining
}

// refillLoop periodically refills tokens
func (rl *RateLimiter) refillLoop() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		rl.refillTokens()
	}
}

// refillTokens refills token bucket
func (rl *RateLimiter) refillTokens() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	// Drain existing tokens
	drained := 0
	for {
		select {
		case <-rl.tokens:
			drained++
		default:
			goto refill
		}
	}

refill:
	// Refill to capacity
	for i := 0; i < rl.requestsPerMinute; i++ {
		select {
		case rl.tokens <- struct{}{}:
		default:
			// Channel full
			break
		}
	}

	rl.lastRefill = time.Now()
}

// GetStats returns rate limiter statistics
func (rl *RateLimiter) GetStats() RateLimiterStats {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	tokensAvailable := len(rl.tokens)
	inBackoff := rl.isInBackoff()
	backoffRemaining := time.Duration(0)

	if inBackoff {
		elapsed := time.Since(rl.lastErrorTime)
		backoffRemaining = rl.backoffDuration - elapsed
	}

	return RateLimiterStats{
		RequestsPerMinute:  rl.requestsPerMinute,
		TokensAvailable:    tokensAvailable,
		ConsecutiveErrors:  rl.consecutiveErrors,
		InBackoff:          inBackoff,
		BackoffRemaining:   backoffRemaining,
		LastRefill:         rl.lastRefill,
	}
}

// RateLimiterStats holds rate limiter statistics
type RateLimiterStats struct {
	RequestsPerMinute int
	TokensAvailable   int
	ConsecutiveErrors int
	InBackoff         bool
	BackoffRemaining  time.Duration
	LastRefill        time.Time
}

// ResetBackoff manually resets backoff state
func (rl *RateLimiter) ResetBackoff() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	rl.consecutiveErrors = 0
	rl.backoffDuration = 0
}

// SetRate changes the rate limit dynamically
func (rl *RateLimiter) SetRate(rpm int) error {
	if rpm <= 0 {
		return fmt.Errorf("rpm must be positive")
	}

	rl.mu.Lock()
	defer rl.mu.Unlock()

	// Create new token channel with new capacity
	oldTokens := rl.tokens
	rl.tokens = make(chan struct{}, rpm)
	rl.requestsPerMinute = rpm

	// Transfer available tokens (up to new capacity)
	transferred := 0
	for transferred < rpm {
		select {
		case <-oldTokens:
			select {
			case rl.tokens <- struct{}{}:
				transferred++
			default:
				goto done
			}
		default:
			goto done
		}
	}

done:
	// Fill remaining capacity
	for i := transferred; i < rpm; i++ {
		select {
		case rl.tokens <- struct{}{}:
		default:
			break
		}
	}

	return nil
}

// Close stops the refill goroutine
func (rl *RateLimiter) Close() {
	// Note: In real implementation, would need a stop channel
	// to cleanly shutdown refillLoop goroutine
}

// RetryConfig holds retry configuration
type RetryConfig struct {
	MaxRetries      int
	InitialBackoff  time.Duration
	MaxBackoff      time.Duration
	BackoffMultiplier float64
}

// DefaultRetryConfig returns default retry configuration
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxRetries:        4,
		InitialBackoff:    2 * time.Second,
		MaxBackoff:        60 * time.Second,
		BackoffMultiplier: 2.0,
	}
}

// RetryWithBackoff executes fn with exponential backoff retry
func RetryWithBackoff(ctx context.Context, rl *RateLimiter, config RetryConfig, fn func() error) error {
	var lastErr error
	backoff := config.InitialBackoff

	for attempt := 0; attempt <= config.MaxRetries; attempt++ {
		// Wait for rate limit token
		if err := rl.Wait(ctx); err != nil {
			return fmt.Errorf("rate limit wait failed: %w", err)
		}

		// Execute function
		err := fn()
		if err == nil {
			rl.RecordSuccess()
			return nil
		}

		lastErr = err
		rl.RecordError()

		// Don't sleep after last attempt
		if attempt == config.MaxRetries {
			break
		}

		// Exponential backoff sleep
		select {
		case <-time.After(backoff):
		case <-ctx.Done():
			return ctx.Err()
		}

		// Increase backoff for next iteration
		backoff = time.Duration(float64(backoff) * config.BackoffMultiplier)
		if backoff > config.MaxBackoff {
			backoff = config.MaxBackoff
		}
	}

	return fmt.Errorf("max retries exceeded: %w", lastErr)
}
