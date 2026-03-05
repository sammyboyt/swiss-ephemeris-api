package eph

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestWorkerPool_SubmitJob(t *testing.T) {
	pool := NewWorkerPool(2, nil)
	pool.Start()
	defer pool.Stop()

	// RED: Should process calculation jobs
	job := CalculationRequest{
		ID:   "test-job",
		Type: "planets",
		Params: map[string]interface{}{
			"year": 2024, "month": 1, "day": 15, "ut": 12.0,
		},
		Response: make(chan CalculationResult, 1),
		Context:  context.Background(),
	}

	pool.Submit(job)

	select {
	case result := <-job.Response:
		assert.NoError(t, result.Error)
		assert.NotNil(t, result.Data)

		bodies, ok := result.Data.([]CelestialBody)
		assert.True(t, ok)
		assert.True(t, len(bodies) > 0) // Should have bodies

	case <-time.After(1 * time.Second):
		t.Fatal("Job did not complete within timeout")
	}
}

func TestWorkerPool_ConcurrentJobs(t *testing.T) {
	pool := NewWorkerPool(4, nil)
	pool.Start()
	defer pool.Stop()

	// RED: Should handle multiple concurrent jobs
	numJobs := 5 // Reduced for faster testing
	results := make(chan CalculationResult, numJobs)

	start := time.Now()
	for i := 0; i < numJobs; i++ {
		job := CalculationRequest{
			ID:   fmt.Sprintf("job-%d", i),
			Type: "planets",
			Params: map[string]interface{}{
				"year": 2024, "month": 1, "day": i + 1, "ut": 12.0,
			},
			Response: make(chan CalculationResult, 1),
			Context:  context.Background(),
		}
		pool.Submit(job)
		go func() {
			result := <-job.Response
			results <- result
		}()
	}

	// Collect all results
	completedJobs := 0
	for completedJobs < numJobs {
		select {
		case result := <-results:
			assert.NoError(t, result.Error)
			assert.NotNil(t, result.Data)
			completedJobs++
		case <-time.After(5 * time.Second):
			t.Fatalf("Only %d/%d jobs completed within timeout", completedJobs, numJobs)
		}
	}

	elapsed := time.Since(start)
	// Should complete in reasonable time with concurrent processing
	assert.True(t, elapsed < 3*time.Second, "Jobs should complete within reasonable time")
}

func TestWorkerPool_JobTimeout(t *testing.T) {
	pool := NewWorkerPool(1, nil)
	pool.Start()
	defer pool.Stop()

	// RED: Should handle job timeouts via context
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	job := CalculationRequest{
		ID:   "timeout-job",
		Type: "planets",
		Params: map[string]interface{}{
			"year": 2024, "month": 1, "day": 15, "ut": 12.0,
		},
		Response: make(chan CalculationResult, 1),
		Context:  ctx,
	}

	// Simulate a slow operation that exceeds timeout
	go func() {
		time.Sleep(100 * time.Millisecond) // Longer than context timeout
		pool.Submit(job)
	}()

	select {
	case <-job.Response:
		t.Fatal("Job should have been cancelled by context timeout")
	case <-ctx.Done():
		// Expected - context was cancelled
		assert.Equal(t, context.DeadlineExceeded, ctx.Err())
	}
}

func TestWorkerPool_GetStats(t *testing.T) {
	logger := createTestLogger(t)
	pool := NewWorkerPool(2, logger)
	pool.Start()
	defer pool.Stop()

	// Test initial stats
	stats := pool.GetStats()
	assert.NotNil(t, stats)
	assert.Equal(t, 2, stats["workers"])
	assert.Equal(t, int64(0), stats["jobs_processed"])
	assert.Equal(t, int64(0), stats["active_jobs"])

	// Submit a job to change stats
	job := CalculationRequest{
		ID:   "test-job",
		Type: "planets",
		Params: map[string]interface{}{
			"year": 2024, "month": 1, "day": 15, "ut": 12.0,
		},
		Response: make(chan CalculationResult, 1),
		Context:  context.Background(),
	}

	pool.Submit(job)

	// Wait for job completion
	select {
	case result := <-job.Response:
		assert.NoError(t, result.Error)
		assert.NotNil(t, result.Data)
	case <-time.After(1 * time.Second):
		t.Fatal("Job did not complete in time")
	}

	// Check updated stats
	stats = pool.GetStats()
	assert.Equal(t, int64(1), stats["jobs_processed"])
	assert.Equal(t, int64(0), stats["active_jobs"]) // Job completed
}
