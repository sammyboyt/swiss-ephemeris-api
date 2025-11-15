package eph

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestCircuitBreaker_Success(t *testing.T) {
	cb := NewCircuitBreaker(3, time.Minute)

	// RED: Should allow calls when healthy
	err := cb.Call(func() error { return nil })
	assert.NoError(t, err)
	assert.Equal(t, StateClosed, cb.GetState())
}

func TestCircuitBreaker_Failure(t *testing.T) {
	cb := NewCircuitBreaker(2, time.Minute)

	// RED: Should open after failure threshold
	_ = cb.Call(func() error { return errors.New("test error") })
	_ = cb.Call(func() error { return errors.New("test error") })

	// Third call should be blocked
	err := cb.Call(func() error { return nil })
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "circuit breaker is open")
	assert.Equal(t, StateOpen, cb.GetState())
}

func TestCircuitBreaker_Recovery(t *testing.T) {
	cb := NewCircuitBreaker(2, 100*time.Millisecond)

	// RED: Should transition to half-open after timeout
	_ = cb.Call(func() error { return errors.New("test error") })
	_ = cb.Call(func() error { return errors.New("test error") })

	assert.Equal(t, StateOpen, cb.GetState())

	// Wait for recovery timeout
	time.Sleep(150 * time.Millisecond)

	// Should allow one test call
	err := cb.Call(func() error { return nil })
	assert.NoError(t, err)
	assert.Equal(t, StateHalfOpen, cb.GetState())

	// Additional successes should close the circuit
	cb.Call(func() error { return nil })
	cb.Call(func() error { return nil })

	assert.Equal(t, StateClosed, cb.GetState())
}

func TestCircuitBreaker_HalfOpenFailure(t *testing.T) {
	cb := NewCircuitBreaker(2, 100*time.Millisecond)

	// Put circuit breaker in half-open state
	_ = cb.Call(func() error { return errors.New("test error") })
	_ = cb.Call(func() error { return errors.New("test error") })
	time.Sleep(150 * time.Millisecond)

	// First call in half-open should succeed
	err := cb.Call(func() error { return nil })
	assert.NoError(t, err)
	assert.Equal(t, StateHalfOpen, cb.GetState())

	// Failure in half-open should reopen circuit
	cb.Call(func() error { return errors.New("half-open failure") })
	assert.Equal(t, StateOpen, cb.GetState())
}

func TestCircuitBreaker_GetStats(t *testing.T) {
	cb := NewCircuitBreaker(3, time.Minute)

	// RED: Should provide statistics
	stats := cb.GetStats()
	assert.NotNil(t, stats)
	assert.Contains(t, stats, "state")
	assert.Contains(t, stats, "failures")
	assert.Contains(t, stats, "successes")
	assert.Equal(t, int64(0), stats["failures"])
	assert.Equal(t, StateClosed, stats["state"])
}

func TestCircuitBreaker_CustomThresholds(t *testing.T) {
	cb := NewCircuitBreaker(5, time.Hour)

	// Should allow 4 failures before opening
	for i := 0; i < 4; i++ {
		_ = cb.Call(func() error { return errors.New("test error") })
		assert.Equal(t, StateClosed, cb.GetState())
	}

	// Fifth failure should open circuit
	_ = cb.Call(func() error { return errors.New("test error") })
	assert.Equal(t, StateOpen, cb.GetState())
}
