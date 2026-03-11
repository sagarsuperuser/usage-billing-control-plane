package replay

import (
	"context"
	"errors"
	"log"
	"time"

	"lago-usage-billing-alpha/internal/store"
)

type Worker struct {
	store        store.Repository
	pollInterval time.Duration
}

func NewWorker(s store.Repository, pollInterval time.Duration) *Worker {
	if pollInterval <= 0 {
		pollInterval = time.Second
	}
	return &Worker{store: s, pollInterval: pollInterval}
}

func (w *Worker) Run(ctx context.Context) {
	ticker := time.NewTicker(w.pollInterval)
	defer ticker.Stop()

	for {
		if !w.processOnce() {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
			continue
		}

		select {
		case <-ctx.Done():
			return
		default:
		}
	}
}

func (w *Worker) processOnce() bool {
	job, err := w.store.DequeueReplayJob()
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return false
		}
		log.Printf("replay worker dequeue failed: %v", err)
		return false
	}

	filter := store.Filter{
		From:       job.From,
		To:         job.To,
		CustomerID: job.CustomerID,
		MeterID:    job.MeterID,
	}
	matchingEvents, err := w.store.ListUsageEvents(filter)
	if err != nil {
		_, _ = w.store.FailReplayJob(job.ID, err.Error(), time.Now().UTC())
		log.Printf("replay worker processing failed for job %s: %v", job.ID, err)
		return true
	}

	_, err = w.store.CompleteReplayJob(job.ID, int64(len(matchingEvents)), time.Now().UTC())
	if err != nil {
		log.Printf("replay worker completion failed for job %s: %v", job.ID, err)
	}

	return true
}
