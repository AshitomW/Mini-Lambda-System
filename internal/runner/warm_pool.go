// Package runner provides a warm execution environment pool for low-latency serverless executions.
package runner

import (
	"context"
	"sync"
	"time"

	"AshitomW/mini-lambda/internal/domain"
)

type warmWorker struct {
	id        string
	fnID      string
	createdAt time.Time
	lastUsed  time.Time
}

// WarmPoolManager maintains warm standby execution environments to mitigate cold start latency.
type WarmPoolManager struct {
	mu             sync.Mutex
	workers        map[string][]*warmWorker
	idleTimeout    time.Duration
	maxWarmPerFunc int
	underlying     ContainerRunner
	stopCleaner    chan struct{}
	closed         bool

	// Telemetry stats
	ColdStarts int64
	WarmStarts int64
}

// NewWarmPoolManager initializes a warm execution pool with the given idle timeout and capacity.
func NewWarmPoolManager(underlying ContainerRunner, idleTimeout time.Duration, maxWarmPerFunc int) *WarmPoolManager {
	if idleTimeout <= 0 {
		idleTimeout = 60 * time.Second
	}
	if maxWarmPerFunc <= 0 {
		maxWarmPerFunc = 5
	}

	p := &WarmPoolManager{
		workers:        make(map[string][]*warmWorker),
		idleTimeout:    idleTimeout,
		maxWarmPerFunc: maxWarmPerFunc,
		underlying:     underlying,
		stopCleaner:    make(chan struct{}),
	}

	go p.runIdleReaper()

	return p
}

// Invoke executes a function, reusing an existing warm worker if available, or initializing a new one.
func (p *WarmPoolManager) Invoke(ctx context.Context, fn domain.Function, payload []byte, invCtx domain.InvocationContext, timeout time.Duration) (*domain.InvocationResult, bool, error) {
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return nil, false, domain.ErrRunnerUnavailable
	}

	poolList := p.workers[fn.ID]
	var isWarm bool
	var worker *warmWorker

	if len(poolList) > 0 {
		// Acquire an existing warm worker
		worker = poolList[len(poolList)-1]
		p.workers[fn.ID] = poolList[:len(poolList)-1]
		isWarm = true
		p.WarmStarts++
	} else {
		// Cold start: allocate a new execution slot
		worker = &warmWorker{
			id:        fn.ID,
			fnID:      fn.ID,
			createdAt: time.Now(),
		}
		p.ColdStarts++
	}
	p.mu.Unlock()

	// Execute through the underlying runner
	res, err := p.underlying.Invoke(ctx, fn, payload, invCtx, timeout)
	if err != nil {
		return nil, isWarm, err
	}

	// Put back into pool if within capacity
	worker.lastUsed = time.Now()
	p.releaseWorker(fn.ID, worker)

	return res, isWarm, nil
}

func (p *WarmPoolManager) releaseWorker(fnID string, w *warmWorker) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		return
	}

	if len(p.workers[fnID]) < p.maxWarmPerFunc {
		p.workers[fnID] = append(p.workers[fnID], w)
	}
}

// GetActiveWarmCount returns the number of idle warm workers currently in standby.
func (p *WarmPoolManager) GetActiveWarmCount(fnID string) int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.workers[fnID])
}

// TotalWarmCount returns total warm workers across all functions.
func (p *WarmPoolManager) TotalWarmCount() int {
	p.mu.Lock()
	defer p.mu.Unlock()

	total := 0
	for _, list := range p.workers {
		total += len(list)
	}
	return total
}

func (p *WarmPoolManager) runIdleReaper() {
	ticker := time.NewTicker(p.idleTimeout / 2)
	defer ticker.Stop()

	for {
		select {
		case <-p.stopCleaner:
			return
		case <-ticker.C:
			p.reapIdleWorkers()
		}
	}
}

func (p *WarmPoolManager) reapIdleWorkers() {
	p.mu.Lock()
	defer p.mu.Unlock()

	now := time.Now()
	for fnID, list := range p.workers {
		var active []*warmWorker
		for _, w := range list {
			if now.Sub(w.lastUsed) < p.idleTimeout {
				active = append(active, w)
			}
		}
		if len(active) == 0 {
			delete(p.workers, fnID)
		} else {
			p.workers[fnID] = active
		}
	}
}

// Close terminates the pool cleaner and flushes all warm workers.
func (p *WarmPoolManager) Close() error {
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return nil
	}
	p.closed = true
	p.workers = make(map[string][]*warmWorker)
	p.mu.Unlock()

	close(p.stopCleaner)
	return nil
}
