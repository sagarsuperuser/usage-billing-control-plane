package service

import (
	"context"
	"fmt"
	"strings"
	"time"
)

type BillingDocumentEmail struct {
	To  []string `json:"to,omitempty"`
	Cc  []string `json:"cc,omitempty"`
	Bcc []string `json:"bcc,omitempty"`
}

type NotificationDispatchError struct {
	StatusCode int
	Backend    string
	Message    string
}

func (e *NotificationDispatchError) Error() string {
	if e == nil {
		return ""
	}
	message := strings.TrimSpace(e.Message)
	if message == "" {
		message = "notification dispatch failed"
	}
	if e.StatusCode > 0 {
		return fmt.Sprintf("%s (backend=%s status=%d)", message, strings.TrimSpace(e.Backend), e.StatusCode)
	}
	if backend := strings.TrimSpace(e.Backend); backend != "" {
		return fmt.Sprintf("%s (backend=%s)", message, backend)
	}
	return message
}

type NotificationDispatchResult struct {
	DispatchedAt time.Time `json:"dispatched_at"`
	Action       string    `json:"action"`
	Domain       string    `json:"domain"`
	Backend      string    `json:"backend"`
}

type BillingNotificationAdapter interface {
	ResendInvoiceEmail(ctx context.Context, invoiceID string, input BillingDocumentEmail) error
	ResendPaymentReceiptEmail(ctx context.Context, paymentReceiptID string, input BillingDocumentEmail) error
	ResendCreditNoteEmail(ctx context.Context, creditNoteID string, input BillingDocumentEmail) error
}

type NotificationService struct {
	workspaceInvitationSender WorkspaceInvitationEmailSender
	passwordResetSender       PasswordResetEmailSender
	paymentSetupRequestSender CustomerPaymentSetupRequestEmailSender
	billingAdapter            BillingNotificationAdapter
}

func NewNotificationService(
	workspaceInvitationSender WorkspaceInvitationEmailSender,
	passwordResetSender PasswordResetEmailSender,
	paymentSetupRequestSender CustomerPaymentSetupRequestEmailSender,
	billingAdapter BillingNotificationAdapter,
) *NotificationService {
	return &NotificationService{
		workspaceInvitationSender: workspaceInvitationSender,
		passwordResetSender:       passwordResetSender,
		paymentSetupRequestSender: paymentSetupRequestSender,
		billingAdapter:            billingAdapter,
	}
}

func (s *NotificationService) CanSendWorkspaceInvitations() bool {
	return s != nil && s.workspaceInvitationSender != nil
}

func (s *NotificationService) CanSendPasswordReset() bool {
	return s != nil && s.passwordResetSender != nil
}

func (s *NotificationService) CanSendCustomerPaymentSetupRequest() bool {
	return s != nil && s.paymentSetupRequestSender != nil
}

func (s *NotificationService) CanResendInvoiceEmail() bool {
	return s != nil && s.billingAdapter != nil
}

func (s *NotificationService) CanResendPaymentReceiptEmail() bool {
	return s != nil && s.billingAdapter != nil
}

func (s *NotificationService) CanResendCreditNoteEmail() bool {
	return s != nil && s.billingAdapter != nil
}

func (s *NotificationService) SendWorkspaceInvitation(input WorkspaceInvitationEmail) error {
	if s == nil || s.workspaceInvitationSender == nil {
		return fmt.Errorf("%w: workspace invitation notification backend is required", ErrValidation)
	}
	return s.workspaceInvitationSender.SendWorkspaceInvitation(input)
}

func (s *NotificationService) SendPasswordReset(input PasswordResetEmail) error {
	if s == nil || s.passwordResetSender == nil {
		return fmt.Errorf("%w: password reset notification backend is required", ErrValidation)
	}
	return s.passwordResetSender.SendPasswordReset(input)
}

func (s *NotificationService) SendCustomerPaymentSetupRequest(input CustomerPaymentSetupRequestEmail) (NotificationDispatchResult, error) {
	if s == nil || s.paymentSetupRequestSender == nil {
		return NotificationDispatchResult{}, fmt.Errorf("%w: payment setup request notification backend is required", ErrValidation)
	}
	if err := s.paymentSetupRequestSender.SendCustomerPaymentSetupRequest(input); err != nil {
		return NotificationDispatchResult{}, err
	}
	return NotificationDispatchResult{
		DispatchedAt: time.Now().UTC(),
		Action:       "send_customer_payment_setup_request",
		Domain:       "product_workflow",
		Backend:      "alpha_email",
	}, nil
}

func (s *NotificationService) ResendInvoiceEmail(ctx context.Context, invoiceID string, input BillingDocumentEmail) (NotificationDispatchResult, error) {
	if s == nil || s.billingAdapter == nil {
		return NotificationDispatchResult{}, fmt.Errorf("%w: billing notification backend is required", ErrValidation)
	}
	invoiceID = strings.TrimSpace(invoiceID)
	if invoiceID == "" {
		return NotificationDispatchResult{}, fmt.Errorf("%w: invoice id is required", ErrValidation)
	}
	if err := s.billingAdapter.ResendInvoiceEmail(ctx, invoiceID, input); err != nil {
		return NotificationDispatchResult{}, err
	}
	return NotificationDispatchResult{
		DispatchedAt: time.Now().UTC(),
		Action:       "resend_invoice_email",
		Domain:       "billing_document",
		Backend:      "lago",
	}, nil
}

func (s *NotificationService) ResendPaymentReceiptEmail(ctx context.Context, paymentReceiptID string, input BillingDocumentEmail) (NotificationDispatchResult, error) {
	if s == nil || s.billingAdapter == nil {
		return NotificationDispatchResult{}, fmt.Errorf("%w: billing notification backend is required", ErrValidation)
	}
	paymentReceiptID = strings.TrimSpace(paymentReceiptID)
	if paymentReceiptID == "" {
		return NotificationDispatchResult{}, fmt.Errorf("%w: payment receipt id is required", ErrValidation)
	}
	if err := s.billingAdapter.ResendPaymentReceiptEmail(ctx, paymentReceiptID, input); err != nil {
		return NotificationDispatchResult{}, err
	}
	return NotificationDispatchResult{
		DispatchedAt: time.Now().UTC(),
		Action:       "resend_payment_receipt_email",
		Domain:       "billing_document",
		Backend:      "lago",
	}, nil
}

func (s *NotificationService) ResendCreditNoteEmail(ctx context.Context, creditNoteID string, input BillingDocumentEmail) (NotificationDispatchResult, error) {
	if s == nil || s.billingAdapter == nil {
		return NotificationDispatchResult{}, fmt.Errorf("%w: billing notification backend is required", ErrValidation)
	}
	creditNoteID = strings.TrimSpace(creditNoteID)
	if creditNoteID == "" {
		return NotificationDispatchResult{}, fmt.Errorf("%w: credit note id is required", ErrValidation)
	}
	if err := s.billingAdapter.ResendCreditNoteEmail(ctx, creditNoteID, input); err != nil {
		return NotificationDispatchResult{}, err
	}
	return NotificationDispatchResult{
		DispatchedAt: time.Now().UTC(),
		Action:       "resend_credit_note_email",
		Domain:       "billing_document",
		Backend:      "lago",
	}, nil
}
