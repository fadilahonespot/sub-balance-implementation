package rest

import (
	"context"
	"sync"
	"time"
)

// CircuitBreakerState represents the state of the circuit breaker
type CircuitBreakerState int

const (
	StateClosed CircuitBreakerState = iota
	StateOpen
	StateHalfOpen
)

// CircuitBreaker implements the circuit breaker pattern
type CircuitBreaker struct {
	maxFailures  int
	resetTimeout time.Duration
	failureCount int
	lastFailTime time.Time
	state        CircuitBreakerState
	mutex        sync.RWMutex
}

// NewCircuitBreaker creates a new circuit breaker
func NewCircuitBreaker(maxFailures int, resetTimeout time.Duration) *CircuitBreaker {
	return &CircuitBreaker{
		maxFailures:  maxFailures,
		resetTimeout: resetTimeout,
		state:        StateClosed,
	}
}

// Execute executes a function with circuit breaker protection
func (cb *CircuitBreaker) Execute(ctx context.Context, fn func() error) error {
	cb.mutex.Lock()
	defer cb.mutex.Unlock()

	// Check if we should allow the request
	if !cb.allowRequest() {
		return ErrCircuitBreakerOpen
	}

	// Execute the function
	err := fn()

	// Update circuit breaker state based on result
	cb.recordResult(err)

	return err
}

// allowRequest checks if a request should be allowed
func (cb *CircuitBreaker) allowRequest() bool {
	switch cb.state {
	case StateClosed:
		return true
	case StateOpen:
		// Check if we should transition to half-open
		if time.Since(cb.lastFailTime) > cb.resetTimeout {
			cb.state = StateHalfOpen
			return true
		}
		return false
	case StateHalfOpen:
		return true
	default:
		return false
	}
}

// recordResult records the result of a request
func (cb *CircuitBreaker) recordResult(err error) {
	if err != nil {
		cb.failureCount++
		cb.lastFailTime = time.Now()

		if cb.failureCount >= cb.maxFailures {
			cb.state = StateOpen
		}
	} else {
		// Success - reset failure count and close circuit if it was half-open
		if cb.state == StateHalfOpen {
			cb.failureCount = 0
			cb.state = StateClosed
		} else if cb.state == StateClosed {
			// Reset failure count on success in closed state
			cb.failureCount = 0
		}
	}
}

// GetState returns the current state of the circuit breaker
func (cb *CircuitBreaker) GetState() CircuitBreakerState {
	cb.mutex.RLock()
	defer cb.mutex.RUnlock()
	return cb.state
}

// GetFailureCount returns the current failure count
func (cb *CircuitBreaker) GetFailureCount() int {
	cb.mutex.RLock()
	defer cb.mutex.RUnlock()
	return cb.failureCount
}

// ErrCircuitBreakerOpen is returned when the circuit breaker is open
var ErrCircuitBreakerOpen = &CircuitBreakerError{
	message: "circuit breaker is open",
}

// CircuitBreakerError represents a circuit breaker error
type CircuitBreakerError struct {
	message string
}

func (e *CircuitBreakerError) Error() string {
	return e.message
}
