package eph

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// CircuitBreakerState represents the state of the circuit breaker
type CircuitBreakerState int

const (
	StateClosed CircuitBreakerState = iota
	StateOpen
	StateHalfOpen
)

// CircuitBreaker implements the circuit breaker pattern for fault tolerance
type CircuitBreaker struct {
	state        int64 // atomic: CircuitBreakerState
	failures     int64 // atomic
	lastFailTime int64 // atomic
	successes    int64 // atomic

	failureThreshold int
	recoveryTimeout  time.Duration
	successThreshold int

	mu sync.RWMutex
}

// NewCircuitBreaker creates a new circuit breaker
func NewCircuitBreaker(failureThreshold int, recoveryTimeout time.Duration) *CircuitBreaker {
	return &CircuitBreaker{
		failureThreshold: failureThreshold,
		recoveryTimeout:  recoveryTimeout,
		successThreshold: 3, // Require 3 successes to close circuit
	}
}

// Call executes the given function with circuit breaker protection
func (cb *CircuitBreaker) Call(fn func() error) error {
	state := CircuitBreakerState(atomic.LoadInt64(&cb.state))

	if state == StateOpen {
		if time.Since(time.Unix(0, atomic.LoadInt64(&cb.lastFailTime))) < cb.recoveryTimeout {
			return fmt.Errorf("circuit breaker is open")
		}

		// Transition to half-open for testing
		atomic.StoreInt64(&cb.state, int64(StateHalfOpen))
		atomic.StoreInt64(&cb.successes, 0)
	}

	err := fn()

	if err != nil {
		cb.recordFailure()
		return err
	}

	cb.recordSuccess()
	return nil
}

// recordFailure increments failure count and potentially opens circuit
func (cb *CircuitBreaker) recordFailure() {
	atomic.AddInt64(&cb.failures, 1)
	atomic.StoreInt64(&cb.lastFailTime, time.Now().UnixNano())

	if atomic.LoadInt64(&cb.failures) >= int64(cb.failureThreshold) {
		atomic.StoreInt64(&cb.state, int64(StateOpen))
	}
}

// recordSuccess increments success count and potentially closes circuit
func (cb *CircuitBreaker) recordSuccess() {
	if CircuitBreakerState(atomic.LoadInt64(&cb.state)) == StateHalfOpen {
		successes := atomic.AddInt64(&cb.successes, 1)
		if successes >= int64(cb.successThreshold) {
			atomic.StoreInt64(&cb.state, int64(StateClosed))
			atomic.StoreInt64(&cb.failures, 0)
		}
	}
}

// GetState returns the current circuit breaker state
func (cb *CircuitBreaker) GetState() CircuitBreakerState {
	return CircuitBreakerState(atomic.LoadInt64(&cb.state))
}

// GetStats returns circuit breaker statistics
func (cb *CircuitBreaker) GetStats() map[string]interface{} {
	return map[string]interface{}{
		"state":             cb.GetState(),
		"failures":          atomic.LoadInt64(&cb.failures),
		"successes":         atomic.LoadInt64(&cb.successes),
		"failure_threshold": cb.failureThreshold,
		"recovery_timeout":  cb.recoveryTimeout.String(),
		"success_threshold": cb.successThreshold,
		"last_failure":      time.Unix(0, atomic.LoadInt64(&cb.lastFailTime)),
	}
}
