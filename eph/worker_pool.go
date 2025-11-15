package eph

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"go.uber.org/zap"
)

// CalculationRequest represents a calculation job
type CalculationRequest struct {
	ID       string
	Type     string // "planets", "houses", "chart"
	Params   map[string]interface{}
	Response chan CalculationResult
	Context  context.Context
}

// CalculationResult holds the result or error
type CalculationResult struct {
	Data  interface{}
	Error error
}

// WorkerPool manages concurrent ephemeris calculations
type WorkerPool struct {
	workers        int
	jobQueue       chan CalculationRequest
	quit           chan bool
	logger         *zap.Logger
	circuitBreaker *CircuitBreaker
	wg             sync.WaitGroup

	// Metrics
	jobsProcessed int64
	jobsFailed    int64
	activeJobs    int64
}

// NewWorkerPool creates a new worker pool
func NewWorkerPool(workers int, logger *zap.Logger) *WorkerPool {
	if workers <= 0 {
		workers = runtime.NumCPU()
	}

	if logger == nil {
		logger = zap.NewNop()
	}

	return &WorkerPool{
		workers:        workers,
		jobQueue:       make(chan CalculationRequest, workers*2),
		quit:           make(chan bool),
		logger:         logger,
		circuitBreaker: NewCircuitBreaker(5, time.Minute),
	}
}

// Start begins processing jobs
func (wp *WorkerPool) Start() {
	for i := 0; i < wp.workers; i++ {
		wp.wg.Add(1)
		go wp.worker(i)
	}
}

// Stop gracefully shuts down the worker pool
func (wp *WorkerPool) Stop() {
	close(wp.quit)
	wp.wg.Wait()
}

// Submit adds a job to the queue
func (wp *WorkerPool) Submit(request CalculationRequest) {
	atomic.AddInt64(&wp.activeJobs, 1)

	select {
	case wp.jobQueue <- request:
		wp.logger.Debug("Job submitted to worker pool",
			zap.String("job_id", request.ID),
			zap.String("type", request.Type))
	case <-request.Context.Done():
		atomic.AddInt64(&wp.activeJobs, -1)
		request.Response <- CalculationResult{
			Error: request.Context.Err(),
		}
	}
}

// worker processes jobs from the queue
func (wp *WorkerPool) worker(id int) {
	defer wp.wg.Done()

	for {
		select {
		case job := <-wp.jobQueue:
			wp.processJob(job)
		case <-wp.quit:
			return
		}
	}
}

// processJob handles the actual calculation
func (wp *WorkerPool) processJob(job CalculationRequest) {
	defer atomic.AddInt64(&wp.activeJobs, -1)

	var result CalculationResult

	// Use circuit breaker to protect against cascading failures
	err := wp.circuitBreaker.Call(func() error {
		return wp.performCalculation(job, &result)
	})

	if err != nil {
		atomic.AddInt64(&wp.jobsFailed, 1)
		result.Error = err
		wp.logger.Error("Job failed",
			zap.String("job_id", job.ID),
			zap.Error(err))
	} else {
		atomic.AddInt64(&wp.jobsProcessed, 1)
	}

	select {
	case job.Response <- result:
		// Result sent successfully
	case <-job.Context.Done():
		wp.logger.Warn("Job context cancelled, discarding result",
			zap.String("job_id", job.ID))
	}
}

// performCalculation executes the actual ephemeris calculation
func (wp *WorkerPool) performCalculation(job CalculationRequest, result *CalculationResult) error {
	switch job.Type {
	case "planets":
		yr := job.Params["year"].(int)
		mon := job.Params["month"].(int)
		day := job.Params["day"].(int)
		ut := job.Params["ut"].(float64)

		planets, err := GetPlanets(yr, mon, day, ut)
		if err != nil {
			return err
		}
		result.Data = planets

	case "houses":
		yr := job.Params["year"].(int)
		mon := job.Params["month"].(int)
		day := job.Params["day"].(int)
		ut := job.Params["ut"].(float64)
		lat := job.Params["lat"].(float64)
		lng := job.Params["lng"].(float64)

		houses := GetHouses(yr, mon, day, ut, lat, lng)
		result.Data = houses

	case "chart":
		yr := job.Params["year"].(int)
		mon := job.Params["month"].(int)
		day := job.Params["day"].(int)
		ut := job.Params["ut"].(float64)
		lat := job.Params["lat"].(float64)
		lng := job.Params["lng"].(float64)

		planets, err := GetPlanets(yr, mon, day, ut)
		if err != nil {
			return err
		}

		houses := GetHouses(yr, mon, day, ut, lat, lng)

		result.Data = map[string]interface{}{
			"planets": planets,
			"houses":  houses,
		}

	default:
		return fmt.Errorf("unknown calculation type: %s", job.Type)
	}

	return nil
}

// GetStats returns worker pool statistics
func (wp *WorkerPool) GetStats() map[string]interface{} {
	return map[string]interface{}{
		"workers":        wp.workers,
		"jobs_processed": atomic.LoadInt64(&wp.jobsProcessed),
		"jobs_failed":    atomic.LoadInt64(&wp.jobsFailed),
		"active_jobs":    atomic.LoadInt64(&wp.activeJobs),
		"queue_length":   len(wp.jobQueue),
		"circuit_state":  wp.circuitBreaker.GetState(),
	}
}
