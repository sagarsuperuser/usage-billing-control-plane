package service

import (
	"bytes"
	"fmt"
	"strings"
	"time"

	"github.com/go-pdf/fpdf"

	"usage-billing-control-plane/internal/domain"
)

// InvoicePDFService generates PDF invoices and optionally stores them in S3.
// Uses go-pdf/fpdf (pure Go, no external dependencies like wkhtmltopdf or Chrome).
type InvoicePDFService struct {
	objectStore ObjectStore // nil = in-memory only (no S3 upload)
}

func NewInvoicePDFService(objectStore ObjectStore) *InvoicePDFService {
	return &InvoicePDFService{objectStore: objectStore}
}

// GenerateAndStore creates a PDF, uploads to S3, and returns the object key.
// If no object store is configured, returns empty key (PDF not persisted).
func (s *InvoicePDFService) GenerateAndStore(invoice domain.Invoice, lineItems []domain.InvoiceLineItem, customerName string) (string, error) {
	pdfBytes, err := s.Generate(invoice, lineItems, customerName)
	if err != nil {
		return "", err
	}

	if s.objectStore == nil {
		return "", nil
	}

	key := fmt.Sprintf("invoices/%s/%s.pdf", invoice.TenantID, invoice.ID)
	if err := s.objectStore.PutObject(nil, key, pdfBytes, "application/pdf"); err != nil {
		return "", fmt.Errorf("upload invoice pdf: %w", err)
	}
	return key, nil
}

// GetDownloadURL returns a presigned URL for an existing PDF.
func (s *InvoicePDFService) GetDownloadURL(objectKey string) (string, error) {
	if s.objectStore == nil || objectKey == "" {
		return "", fmt.Errorf("pdf not available")
	}
	return s.objectStore.PresignGetObject(nil, objectKey, 1*time.Hour)
}

// Generate creates a PDF for the given invoice and line items, returning raw bytes.
func (s *InvoicePDFService) Generate(invoice domain.Invoice, lineItems []domain.InvoiceLineItem, customerName string) ([]byte, error) {
	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.SetAutoPageBreak(true, 20)
	pdf.AddPage()

	// -- Header --
	pdf.SetFont("Helvetica", "B", 20)
	pdf.SetTextColor(30, 30, 30)
	pdf.CellFormat(0, 12, "INVOICE", "", 1, "L", false, 0, "")

	pdf.SetFont("Helvetica", "", 10)
	pdf.SetTextColor(100, 100, 100)
	pdf.CellFormat(0, 6, fmt.Sprintf("#%s", invoice.InvoiceNumber), "", 1, "L", false, 0, "")
	pdf.Ln(4)

	// -- Invoice metadata --
	pdf.SetFont("Helvetica", "", 9)
	pdf.SetTextColor(80, 80, 80)

	metaLeft := []struct{ label, value string }{
		{"Customer", customerName},
		{"Status", strings.ToUpper(string(invoice.Status))},
		{"Currency", strings.ToUpper(invoice.Currency)},
		{"Billing period", fmt.Sprintf("%s — %s", formatDate(invoice.BillingPeriodStart), formatDate(invoice.BillingPeriodEnd))},
	}

	metaRight := []struct{ label, value string }{
		{"Issued", formatDatePtr(invoice.IssuedAt)},
		{"Due", formatDatePtr(invoice.DueAt)},
		{"Payment terms", fmt.Sprintf("Net %d days", invoice.NetPaymentTermDays)},
	}

	startY := pdf.GetY()
	for _, m := range metaLeft {
		pdf.SetFont("Helvetica", "", 8)
		pdf.SetTextColor(140, 140, 140)
		pdf.CellFormat(35, 5, m.label, "", 0, "L", false, 0, "")
		pdf.SetFont("Helvetica", "", 9)
		pdf.SetTextColor(50, 50, 50)
		pdf.CellFormat(60, 5, m.value, "", 1, "L", false, 0, "")
	}

	pdf.SetY(startY)
	for _, m := range metaRight {
		pdf.SetX(110)
		pdf.SetFont("Helvetica", "", 8)
		pdf.SetTextColor(140, 140, 140)
		pdf.CellFormat(30, 5, m.label, "", 0, "L", false, 0, "")
		pdf.SetFont("Helvetica", "", 9)
		pdf.SetTextColor(50, 50, 50)
		pdf.CellFormat(50, 5, m.value, "", 1, "L", false, 0, "")
	}

	pdf.Ln(8)

	// -- Line items table --
	pdf.SetFillColor(245, 247, 250)
	pdf.SetFont("Helvetica", "B", 8)
	pdf.SetTextColor(100, 100, 100)

	colWidths := []float64{60, 30, 30, 25, 25, 25}
	headers := []string{"Description", "Type", "Qty", "Amount", "Tax", "Total"}
	for i, h := range headers {
		pdf.CellFormat(colWidths[i], 7, h, "", 0, "L", true, 0, "")
	}
	pdf.Ln(-1)

	// Separator line
	pdf.SetDrawColor(220, 220, 220)
	pdf.Line(10, pdf.GetY(), 200, pdf.GetY())

	pdf.SetFont("Helvetica", "", 9)
	pdf.SetTextColor(40, 40, 40)

	for _, item := range lineItems {
		if item.LineType == domain.LineTypeTax {
			continue // taxes shown in totals
		}

		desc := item.Description
		if len(desc) > 35 {
			desc = desc[:32] + "..."
		}

		pdf.CellFormat(colWidths[0], 6, desc, "", 0, "L", false, 0, "")
		pdf.CellFormat(colWidths[1], 6, string(item.LineType), "", 0, "L", false, 0, "")
		pdf.CellFormat(colWidths[2], 6, fmt.Sprintf("%d", item.Quantity), "", 0, "R", false, 0, "")
		pdf.CellFormat(colWidths[3], 6, formatCents(item.AmountCents), "", 0, "R", false, 0, "")
		pdf.CellFormat(colWidths[4], 6, formatCents(item.TaxAmountCents), "", 0, "R", false, 0, "")
		pdf.CellFormat(colWidths[5], 6, formatCents(item.TotalAmountCents), "", 1, "R", false, 0, "")
	}

	pdf.Ln(4)
	pdf.SetDrawColor(220, 220, 220)
	pdf.Line(10, pdf.GetY(), 200, pdf.GetY())
	pdf.Ln(4)

	// -- Totals --
	totalsX := 130.0
	pdf.SetFont("Helvetica", "", 9)
	pdf.SetTextColor(80, 80, 80)

	totals := []struct{ label, value string }{
		{"Subtotal", formatCents(invoice.SubtotalCents)},
		{"Discount", formatCents(-invoice.DiscountCents)},
		{"Tax", formatCents(invoice.TaxAmountCents)},
	}

	for _, t := range totals {
		pdf.SetX(totalsX)
		pdf.CellFormat(35, 6, t.label, "", 0, "L", false, 0, "")
		pdf.CellFormat(30, 6, t.value, "", 1, "R", false, 0, "")
	}

	pdf.Ln(2)
	pdf.SetX(totalsX)
	pdf.SetFont("Helvetica", "B", 11)
	pdf.SetTextColor(30, 30, 30)
	pdf.CellFormat(35, 8, "Total due", "", 0, "L", false, 0, "")
	pdf.CellFormat(30, 8, formatCents(invoice.TotalAmountCents), "", 1, "R", false, 0, "")

	// -- Footer --
	if invoice.Memo != "" {
		pdf.Ln(10)
		pdf.SetFont("Helvetica", "", 8)
		pdf.SetTextColor(120, 120, 120)
		pdf.MultiCell(0, 4, invoice.Memo, "", "L", false)
	}

	if invoice.Footer != "" {
		pdf.Ln(4)
		pdf.SetFont("Helvetica", "", 7)
		pdf.SetTextColor(160, 160, 160)
		pdf.MultiCell(0, 4, invoice.Footer, "", "L", false)
	}

	// -- Generated timestamp --
	pdf.Ln(8)
	pdf.SetFont("Helvetica", "", 7)
	pdf.SetTextColor(180, 180, 180)
	pdf.CellFormat(0, 4, fmt.Sprintf("Generated %s · Alpha Billing", time.Now().UTC().Format("2006-01-02 15:04 UTC")), "", 0, "L", false, 0, "")

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, fmt.Errorf("generate pdf: %w", err)
	}
	return buf.Bytes(), nil
}

func formatCents(cents int64) string {
	negative := cents < 0
	if negative {
		cents = -cents
	}
	dollars := cents / 100
	remainder := cents % 100
	s := fmt.Sprintf("%d.%02d", dollars, remainder)
	if negative {
		return "-" + s
	}
	return s
}

func formatDate(t time.Time) string {
	if t.IsZero() {
		return "—"
	}
	return t.Format("Jan 2, 2006")
}

func formatDatePtr(t *time.Time) string {
	if t == nil {
		return "—"
	}
	return formatDate(*t)
}
