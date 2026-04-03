package service

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// InvoiceNumberGenerator produces sequential, tenant-scoped invoice numbers.
// Format: {prefix}-{YYYY}-{sequence} where prefix defaults to "INV".
type InvoiceNumberGenerator struct {
	db *sql.DB
}

func NewInvoiceNumberGenerator(db *sql.DB) *InvoiceNumberGenerator {
	return &InvoiceNumberGenerator{db: db}
}

// Next returns the next invoice number for the tenant, holding a row-level
// lock to prevent duplicate numbers under concurrent billing cycles.
func (g *InvoiceNumberGenerator) Next(ctx context.Context, tenantID, prefix string) (string, error) {
	if prefix == "" {
		prefix = "INV"
	}
	year := time.Now().UTC().Year()

	tx, err := g.db.BeginTx(ctx, nil)
	if err != nil {
		return "", fmt.Errorf("invoice number tx: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	// Count existing invoices for this tenant+year to determine next sequence.
	// FOR UPDATE on the count query serializes concurrent generators.
	var seq int
	err = tx.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM invoices
		WHERE tenant_id = $1 AND invoice_number LIKE $2
		FOR UPDATE`, tenantID, fmt.Sprintf("%s-%d-%%", prefix, year)).Scan(&seq)
	if err != nil {
		return "", fmt.Errorf("invoice number count: %w", err)
	}

	number := fmt.Sprintf("%s-%d-%04d", prefix, year, seq+1)

	if err := tx.Commit(); err != nil {
		return "", fmt.Errorf("invoice number commit: %w", err)
	}
	return number, nil
}
