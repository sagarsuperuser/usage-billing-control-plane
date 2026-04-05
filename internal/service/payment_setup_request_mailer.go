package service

import (
	"fmt"
	"strings"
)

type CustomerPaymentSetupRequestEmail struct {
	ToEmail          string
	CustomerName     string
	WorkspaceName    string
	CheckoutURL      string
	RequestedByEmail string
	RequestKind      string
}

type CustomerPaymentSetupRequestEmailSender interface {
	SendCustomerPaymentSetupRequest(input CustomerPaymentSetupRequestEmail) error
}

// SMTPPaymentSetupRequestEmailSender sends payment setup request emails via a shared SMTPMailer.
type SMTPPaymentSetupRequestEmailSender struct {
	mailer *SMTPMailer
}

func NewSMTPPaymentSetupRequestEmailSender(mailer *SMTPMailer) *SMTPPaymentSetupRequestEmailSender {
	return &SMTPPaymentSetupRequestEmailSender{mailer: mailer}
}

func (s *SMTPPaymentSetupRequestEmailSender) SendCustomerPaymentSetupRequest(input CustomerPaymentSetupRequestEmail) error {
	if s == nil || s.mailer == nil {
		return fmt.Errorf("payment setup request email sender is not configured")
	}
	toEmail := strings.ToLower(strings.TrimSpace(input.ToEmail))
	if toEmail == "" {
		return fmt.Errorf("payment setup request recipient email is required")
	}
	checkoutURL := strings.TrimSpace(input.CheckoutURL)
	if checkoutURL == "" {
		return fmt.Errorf("payment setup checkout url is required")
	}
	customerName := strings.TrimSpace(input.CustomerName)
	if customerName == "" {
		customerName = "your account"
	}
	workspaceName := strings.TrimSpace(input.WorkspaceName)
	if workspaceName == "" {
		workspaceName = "Alpha"
	}
	_, err := s.mailer.Send(SMTPMessage{
		To:      toEmail,
		Subject: fmt.Sprintf("Complete your payment setup for %s", workspaceName),
		Body:    buildCustomerPaymentSetupRequestEmailBody(customerName, workspaceName, checkoutURL, input.RequestedByEmail, input.RequestKind),
	})
	return err
}

func buildCustomerPaymentSetupRequestEmailBody(customerName, workspaceName, checkoutURL, requestedByEmail, requestKind string) string {
	verb := "requested"
	if strings.EqualFold(strings.TrimSpace(requestKind), "resent") {
		verb = "resent"
	}
	lines := []string{
		fmt.Sprintf("A payment setup link has been %s for %s in %s.", verb, customerName, workspaceName),
		"",
		"Open this secure hosted link to add or verify your payment method:",
		checkoutURL,
	}
	if requestedBy := strings.TrimSpace(requestedByEmail); requestedBy != "" {
		lines = append(lines, "", fmt.Sprintf("Requested by: %s", requestedBy))
	}
	lines = append(lines,
		"",
		"Alpha never asks operators to collect your card details directly.",
		"If you were not expecting this request, you can ignore this email.",
	)
	return strings.Join(lines, "\r\n")
}
