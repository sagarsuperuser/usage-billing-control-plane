package replay

import (
	"errors"
	"time"

	"lago-usage-billing-alpha/internal/domain"
	"lago-usage-billing-alpha/internal/store"
)

type Processor struct {
	store store.Repository
}

func NewProcessor(s store.Repository) *Processor {
	return &Processor{store: s}
}

func (p *Processor) ProcessJobByID(tenantID, id string) (domain.ReplayJob, int64, int64, error) {
	job, err := p.store.StartReplayJob(tenantID, id)
	if err != nil {
		if errors.Is(err, store.ErrInvalidState) {
			existing, getErr := p.store.GetReplayJob(tenantID, id)
			if getErr == nil {
				switch existing.Status {
				case domain.ReplayRunning, domain.ReplayDone, domain.ReplayFailed:
					return existing, 0, 0, nil
				}
			}
		}
		return domain.ReplayJob{}, 0, 0, err
	}

	processed, adjustments, err := p.ProcessClaimedJob(job)
	return job, processed, adjustments, err
}

func (p *Processor) ProcessClaimedJob(job domain.ReplayJob) (int64, int64, error) {
	filter := store.Filter{
		TenantID:   job.TenantID,
		From:       job.From,
		To:         job.To,
		CustomerID: job.CustomerID,
		MeterID:    job.MeterID,
	}

	matchingEvents, err := p.store.ListUsageEvents(filter)
	if err != nil {
		return 0, 0, p.fail(job.ID, err)
	}

	matchingBilled, err := p.store.ListBilledEntries(filter)
	if err != nil {
		return 0, 0, p.fail(job.ID, err)
	}

	adjustments, err := p.applyAdjustments(job, matchingEvents, matchingBilled)
	if err != nil {
		return 0, adjustments, p.fail(job.ID, err)
	}

	processed := int64(len(matchingEvents))
	if _, err := p.store.CompleteReplayJob(job.ID, processed, time.Now().UTC()); err != nil {
		return processed, adjustments, p.fail(job.ID, err)
	}

	return processed, adjustments, nil
}

type replayAggregate struct {
	customerID string
	meterID    string
	quantity   int64
	billed     int64
}

func (p *Processor) applyAdjustments(job domain.ReplayJob, events []domain.UsageEvent, billed []domain.BilledEntry) (int64, error) {
	rows := make(map[string]*replayAggregate)

	for _, event := range events {
		key := event.CustomerID + "::" + event.MeterID
		agg, ok := rows[key]
		if !ok {
			agg = &replayAggregate{
				customerID: event.CustomerID,
				meterID:    event.MeterID,
			}
			rows[key] = agg
		}
		agg.quantity += event.Quantity
	}

	for _, entry := range billed {
		key := entry.CustomerID + "::" + entry.MeterID
		agg, ok := rows[key]
		if !ok {
			agg = &replayAggregate{
				customerID: entry.CustomerID,
				meterID:    entry.MeterID,
			}
			rows[key] = agg
		}
		agg.billed += entry.AmountCents
	}

	now := time.Now().UTC()
	adjustments := int64(0)
	for _, agg := range rows {
		meter, err := p.store.GetMeter(job.TenantID, agg.meterID)
		if err != nil {
			return adjustments, err
		}
		rule, err := p.store.GetRatingRuleVersion(job.TenantID, meter.RatingRuleVersionID)
		if err != nil {
			return adjustments, err
		}

		computed, err := domain.ComputeAmountCents(rule, agg.quantity)
		if err != nil {
			return adjustments, err
		}
		delta := computed - agg.billed
		if delta == 0 {
			continue
		}

		_, err = p.store.CreateBilledEntry(domain.BilledEntry{
			TenantID:    job.TenantID,
			CustomerID:  agg.customerID,
			MeterID:     agg.meterID,
			AmountCents: delta,
			Source:      domain.BilledEntrySourceReplayAdjustment,
			ReplayJobID: job.ID,
			Timestamp:   now,
		})
		if err != nil {
			return adjustments, err
		}
		adjustments++
	}

	return adjustments, nil
}

func (p *Processor) fail(jobID string, cause error) error {
	if cause == nil {
		return nil
	}
	_, _ = p.store.FailReplayJob(jobID, cause.Error(), time.Now().UTC())
	return cause
}
