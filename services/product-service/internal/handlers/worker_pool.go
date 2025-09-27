package handlers

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"
)

// Request represents a generic request that can be processed by workers
type Request struct {
	ID        string
	Type      string
	Data      interface{}
	Context   context.Context
	Response  chan Response
	Timestamp time.Time
}

// Response represents the response from a worker
type Response struct {
	ID       string
	Data     interface{}
	Error    error
	Duration time.Duration
}

// WorkerPool manages a pool of workers to handle requests
type WorkerPool struct {
	workers    int
	requestCh  chan Request
	quitCh     chan bool
	wg         sync.WaitGroup
	ctx        context.Context
	cancel     context.CancelFunc
	activeJobs int64
	mu         sync.RWMutex
	
	// Custom handlers
	handleGetProducts    func(Request) Response
	handleGetProductByID func(Request) Response
}

// NewWorkerPool creates a new worker pool with the specified number of workers
func NewWorkerPool(workers int) *WorkerPool {
	ctx, cancel := context.WithCancel(context.Background())
	
	return &WorkerPool{
		workers:   workers,
		requestCh: make(chan Request, workers*2), // Buffer for 2x workers
		quitCh:    make(chan bool),
		ctx:       ctx,
		cancel:    cancel,
	}
}

// Start initializes and starts the worker pool
func (wp *WorkerPool) Start() {
	log.Printf("Starting worker pool with %d workers", wp.workers)
	
	for i := 0; i < wp.workers; i++ {
		wp.wg.Add(1)
		go wp.worker(i)
	}
}

// Stop gracefully shuts down the worker pool
func (wp *WorkerPool) Stop() {
	log.Println("Stopping worker pool...")
	
	// Cancel context to signal workers to stop
	wp.cancel()
	
	// Close request channel
	close(wp.requestCh)
	
	// Wait for all workers to finish
	wp.wg.Wait()
	
	log.Println("Worker pool stopped")
}

// SubmitRequest submits a request to the worker pool
func (wp *WorkerPool) SubmitRequest(req Request) error {
	select {
	case wp.requestCh <- req:
		wp.mu.Lock()
		wp.activeJobs++
		wp.mu.Unlock()
		return nil
	case <-wp.ctx.Done():
		return fmt.Errorf("worker pool is shutting down")
	default:
		return fmt.Errorf("worker pool is full, request rejected")
	}
}

// GetActiveJobs returns the number of active jobs
func (wp *WorkerPool) GetActiveJobs() int64 {
	wp.mu.RLock()
	defer wp.mu.RUnlock()
	return wp.activeJobs
}

// worker is the main worker function that processes requests
func (wp *WorkerPool) worker(id int) {
	defer wp.wg.Done()
	
	log.Printf("Worker %d started", id)
	
	for {
		select {
		case req, ok := <-wp.requestCh:
			if !ok {
				log.Printf("Worker %d: request channel closed, stopping", id)
				return
			}
			
			wp.processRequest(id, req)
			
		case <-wp.ctx.Done():
			log.Printf("Worker %d: context cancelled, stopping", id)
			return
		}
	}
}

// processRequest processes a single request
func (wp *WorkerPool) processRequest(workerID int, req Request) {
	start := time.Now()
	
	log.Printf("Worker %d: processing request %s of type %s", workerID, req.ID, req.Type)
	
	// Check if request context is already cancelled
	select {
	case <-req.Context.Done():
		req.Response <- Response{
			ID:       req.ID,
			Data:     nil,
			Error:    fmt.Errorf("request context cancelled"),
			Duration: time.Since(start),
		}
		wp.decrementActiveJobs()
		return
	default:
	}
	
	// Process the request based on type
	var response Response
	switch req.Type {
	case "get_products":
		if wp.handleGetProducts != nil {
			response = wp.handleGetProducts(req)
		} else {
			response = Response{
				ID:       req.ID,
				Data:     nil,
				Error:    fmt.Errorf("get products handler not set"),
				Duration: time.Since(start),
			}
		}
	case "get_product_by_id":
		if wp.handleGetProductByID != nil {
			response = wp.handleGetProductByID(req)
		} else {
			response = Response{
				ID:       req.ID,
				Data:     nil,
				Error:    fmt.Errorf("get product by id handler not set"),
				Duration: time.Since(start),
			}
		}
	default:
		response = Response{
			ID:       req.ID,
			Data:     nil,
			Error:    fmt.Errorf("unknown request type: %s", req.Type),
			Duration: time.Since(start),
		}
	}
	
	// Send response
	select {
	case req.Response <- response:
		log.Printf("Worker %d: sent response for request %s in %v", workerID, req.ID, response.Duration)
	case <-req.Context.Done():
		log.Printf("Worker %d: request context cancelled while sending response", workerID)
	}
	
	wp.decrementActiveJobs()
}

// decrementActiveJobs safely decrements the active jobs counter
func (wp *WorkerPool) decrementActiveJobs() {
	wp.mu.Lock()
	wp.activeJobs--
	wp.mu.Unlock()
}

