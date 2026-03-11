package reconcile

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"sort"
	"time"

	"lago-usage-billing-alpha/internal/domain"
	"lago-usage-billing-alpha/internal/store"
)

type Service struct {
	store store.Repository
}

type Filter struct {
	CustomerID string
	From       *time.Time
	To         *time.Time
}

func NewService(s store.Repository) *Service {
	return &Service{store: s}
}

func (s *Service) GenerateReport(filter Filter) (domain.ReconciliationReport, error) {
	events, err := s.store.ListUsageEvents(store.Filter{
		From:       filter.From,
		To:         filter.To,
		CustomerID: filter.CustomerID,
	})
	if err != nil {
		return domain.ReconciliationReport{}, err
	}

	billed, err := s.store.ListBilledEntries(store.Filter{
		From:       filter.From,
		To:         filter.To,
		CustomerID: filter.CustomerID,
	})
	if err != nil {
		return domain.ReconciliationReport{}, err
	}

	type aggregate struct {
		customerID    string
		meterID       string
		eventQuantity int64
		computedCents int64
		billedCents   int64
	}

	rowsMap := make(map[string]*aggregate)

	for _, event := range events {
		key := event.CustomerID + "::" + event.MeterID
		agg, ok := rowsMap[key]
		if !ok {
			agg = &aggregate{customerID: event.CustomerID, meterID: event.MeterID}
			rowsMap[key] = agg
		}
		agg.eventQuantity += event.Quantity
	}

	for _, bill := range billed {
		key := bill.CustomerID + "::" + bill.MeterID
		agg, ok := rowsMap[key]
		if !ok {
			agg = &aggregate{customerID: bill.CustomerID, meterID: bill.MeterID}
			rowsMap[key] = agg
		}
		agg.billedCents += bill.AmountCents
	}

	report := domain.ReconciliationReport{GeneratedAt: time.Now().UTC()}
	report.Rows = make([]domain.ReconciliationRow, 0, len(rowsMap))

	for _, agg := range rowsMap {
		meter, err := s.store.GetMeter(agg.meterID)
		if err != nil {
			return domain.ReconciliationReport{}, fmt.Errorf("meter %s not found", agg.meterID)
		}
		rule, err := s.store.GetRatingRuleVersion(meter.RatingRuleVersionID)
		if err != nil {
			return domain.ReconciliationReport{}, fmt.Errorf("rating rule for meter %s not found", agg.meterID)
		}
		computed, err := domain.ComputeAmountCents(rule, agg.eventQuantity)
		if err != nil {
			return domain.ReconciliationReport{}, fmt.Errorf("compute amount failed for meter %s", agg.meterID)
		}
		agg.computedCents = computed
		delta := computed - agg.billedCents
		row := domain.ReconciliationRow{
			CustomerID:          agg.customerID,
			MeterID:             agg.meterID,
			EventQuantity:       agg.eventQuantity,
			ComputedAmountCents: agg.computedCents,
			BilledAmountCents:   agg.billedCents,
			DeltaCents:          delta,
			Mismatch:            delta != 0,
		}
		report.Rows = append(report.Rows, row)
		report.TotalComputedCents += agg.computedCents
		report.TotalBilledCents += agg.billedCents
		report.TotalDeltaCents += delta
		if row.Mismatch {
			report.MismatchRowCount++
		}
	}

	sort.Slice(report.Rows, func(i, j int) bool {
		if report.Rows[i].CustomerID == report.Rows[j].CustomerID {
			return report.Rows[i].MeterID < report.Rows[j].MeterID
		}
		return report.Rows[i].CustomerID < report.Rows[j].CustomerID
	})

	return report, nil
}

func (s *Service) GenerateCSV(report domain.ReconciliationReport) (string, error) {
	var buf bytes.Buffer
	writer := csv.NewWriter(&buf)

	header := []string{"customer_id", "meter_id", "event_quantity", "computed_amount_cents", "billed_amount_cents", "delta_cents", "mismatch"}
	if err := writer.Write(header); err != nil {
		return "", err
	}

	for _, row := range report.Rows {
		record := []string{
			row.CustomerID,
			row.MeterID,
			fmt.Sprintf("%d", row.EventQuantity),
			fmt.Sprintf("%d", row.ComputedAmountCents),
			fmt.Sprintf("%d", row.BilledAmountCents),
			fmt.Sprintf("%d", row.DeltaCents),
			fmt.Sprintf("%t", row.Mismatch),
		}
		if err := writer.Write(record); err != nil {
			return "", err
		}
	}

	writer.Flush()
	if err := writer.Error(); err != nil {
		return "", err
	}

	return buf.String(), nil
}
