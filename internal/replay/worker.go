package replay

import (
	"context"
	"errors"
	"log"
	"sync/atomic"
	"time"

	"lago-usage-billing-alpha/internal/store"
)

const (
	defaultErrorBackoffMin = 250 * time.Millisecond
	defaultErrorBackoffMax = 5 * time.Second
)

type WorkerStats struct {
	DequeuedTotal      int64 `json:"dequeued_total"`
	ProcessedTotal     int64 `json:"processed_total"`
	FailedTotal        int64 `json:"failed_total"`
	DequeueErrorsTotal int64 `json:"dequeue_errors_total"`
	EmptyPollsTotal    int64 `json:"empty_polls_total"`
}

type Worker struct {
	store           store.Repository
	pollInterval    time.Duration
	errorBackoffMin time.Duration
	errorBackoffMax time.Duration

	dequeuedTotal      atomic.Int64
	processedTotal     atomic.Int64
	failedTotal        atomic.Int64
	dequeueErrorsTotal atomic.Int64
	emptyPollsTotal    atomic.Int64
}

type WorkerOption func(*Worker)

func WithErrorBackoff(min, max time.Duration) WorkerOption {
	return func(w *Worker) {
		if min > 0 {
			w.errorBackoffMin = min
		}
		if max > 0 {
			w.errorBackoffMax = max
		}
		if w.errorBackoffMax < w.errorBackoffMin {
			w.errorBackoffMax = w.errorBackoffMin
		}
	}
}

func NewWorker(s store.Repository, pollInterval time.Duration, opts ...WorkerOption) *Worker {
	if pollInterval <= 0 {
		pollInterval = time.Second
	}
	w := &Worker{
		store:           s,
		pollInterval:    pollInterval,
		errorBackoffMin: defaultErrorBackoffMin,
		errorBackoffMax: defaultErrorBackoffMax,
	}
	for _, opt := range opts {
		opt(w)
	}
	return w
}

func (w *Worker) Run(ctx context.Context) {
	backoff := w.errorBackoffMin

	for {
		result := w.processOnce()

		switch result {
		case processResultIdle:
			w.emptyPollsTotal.Add(1)
			if !sleepWithContext(ctx, w.pollInterval) {
				return
			}
		case processResultError:
			if !sleepWithContext(ctx, backoff) {
				return
			}
			backoff *= 2
			if backoff > w.errorBackoffMax {
				backoff = w.errorBackoffMax
			}
		case processResultProcessed:
			backoff = w.errorBackoffMin
			select {
			case <-ctx.Done():
				return
			default:
			}
		}
	}
}

func (w *Worker) Stats() WorkerStats {
	return WorkerStats{
		DequeuedTotal:      w.dequeuedTotal.Load(),
		ProcessedTotal:     w.processedTotal.Load(),
		FailedTotal:        w.failedTotal.Load(),
		DequeueErrorsTotal: w.dequeueErrorsTotal.Load(),
		EmptyPollsTotal:    w.emptyPollsTotal.Load(),
	}
}

type processResult int

const (
	processResultIdle processResult = iota
	processResultProcessed
	processResultError
)

func (w *Worker) processOnce() processResult {
	job, err := w.store.DequeueReplayJob()
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return processResultIdle
		}
		w.dequeueErrorsTotal.Add(1)
		log.Printf("level=error component=replay_worker event=dequeue_failed err=%q", err.Error())
		return processResultError
	}

	w.dequeuedTotal.Add(1)
	started := time.Now().UTC()

	filter := store.Filter{
		From:       job.From,
		To:         job.To,
		CustomerID: job.CustomerID,
		MeterID:    job.MeterID,
	}

	matchingEvents, err := w.store.ListUsageEvents(filter)
	if err != nil {
		_, _ = w.store.FailReplayJob(job.ID, err.Error(), time.Now().UTC())
		w.failedTotal.Add(1)
		log.Printf("level=error component=replay_worker event=job_failed job_id=%s err=%q", job.ID, err.Error())
		return processResultProcessed
	}

	processed := int64(len(matchingEvents))
	if _, err := w.store.CompleteReplayJob(job.ID, processed, time.Now().UTC()); err != nil {
		_, _ = w.store.FailReplayJob(job.ID, err.Error(), time.Now().UTC())
		w.failedTotal.Add(1)
		log.Printf("level=error component=replay_worker event=completion_failed job_id=%s err=%q", job.ID, err.Error())
		return processResultProcessed
	}

	w.processedTotal.Add(1)
	durationMs := time.Since(started).Milliseconds()
	log.Printf("level=info component=replay_worker event=job_completed job_id=%s processed_records=%d duration_ms=%d", job.ID, processed, durationMs)
	return processResultProcessed
}

func sleepWithContext(ctx context.Context, d time.Duration) bool {
	if d <= 0 {
		return true
	}
	timer := time.NewTimer(d)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}
