package main

import (
	"context"
	"fmt"
	"sync"
	"time"
)

func produce(ctx context.Context, jobs chan<- string) {
	defer close(jobs)

	eventIDs := []string{"evt-001", "evt-002", "evt-003", "evt-004", "evt-005"}

	for _, eventID := range eventIDs {
		select {
		case jobs <- eventID:
			fmt.Println("produced:", eventID)

		case <-ctx.Done():
			fmt.Println("producer stopped:", ctx.Err())
			return
		}
	}
}

func consume(ctx context.Context, workerID int, jobs <-chan string, wg *sync.WaitGroup) {
	defer wg.Done()

	for {
		select {
		case job, ok := <-jobs:
			if !ok {
				fmt.Println("worker stopped, jobs channel closed:", workerID)
				return
			}

			fmt.Println("worker", workerID, "processing:", job)
			time.Sleep(500 * time.Millisecond)

		case <-ctx.Done():
			fmt.Println("worker stopped by context:", workerID, ctx.Err())
			return
		}
	}
}

func runDemo() {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	jobs := make(chan string, 5)

	go produce(ctx, jobs)

	var wg sync.WaitGroup

	for workerID := 1; workerID <= 3; workerID++ {
		wg.Add(1)
		go consume(ctx, workerID, jobs, &wg)
	}

	wg.Wait()
	fmt.Println("concurrency demo finished")
}
