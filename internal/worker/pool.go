package worker

import (
	"context"
	"sync"

	"github.com/kadriyebarlak/notification-dispatcher/internal/domain"
)

type WorkerPool struct {
	jobs        chan domain.NotificationEvent
	workerCount int
	wg          sync.WaitGroup
}

func NewWorkerPool(workerCount int) *WorkerPool {
	return &WorkerPool{
		jobs:        make(chan domain.NotificationEvent, workerCount*2),
		workerCount: workerCount,
	}
}

func (p *WorkerPool) Start(ctx context.Context, process func(ctx context.Context, event domain.NotificationEvent)) {
	for i := 0; i < p.workerCount; i++ {
		p.wg.Add(1)
		go func() {
			defer p.wg.Done()
			for {
				select {
				case event, ok := <-p.jobs:
					if !ok {
						return
					}
					process(ctx, event)
				case <-ctx.Done():
					return
				}
			}
		}()
	}
}

func (p *WorkerPool) Stop() {
	close(p.jobs)
	p.wg.Wait()
}

func (p *WorkerPool) Submit(event domain.NotificationEvent) {
	p.jobs <- event
}
