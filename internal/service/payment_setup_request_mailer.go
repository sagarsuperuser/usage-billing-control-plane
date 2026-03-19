package service

import (
	"bytes"
	"fmt"
	"net/smtp"
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

type SMTPPaymentSetupRequestEmailConfig struct {
	Host      string
	Port      int
	Username  string
	Password  string
	FromEmail string
	FromName  string
}

type SMTPPaymentSetupRequestEmailSender struct {
	host      string
	port      int
	username  string
	password  string
	fromEmail string
	fromName  string
}

func NewSMTPPaymentSetupRequestEmailSender(cfg SMTPPaymentSetupRequestEmailConfig) (*SMTPPaymentSetupRequestEmailSender, error) {
	host := strings.TrimSpace(cfg.Host)
	fromEmail := strings.TrimSpace(cfg.FromEmail)
	if host == "" {
		return nil, fmt.Errorf("smtp host is required")
	}
	if cfg.Port <= 0 {
		return nil, fmt.Errorf("smtp port is required")
	}
	if fromEmail == "" {
		return nil, fmt.Errorf("smtp from email is required")
	}
	return &SMTPPaymentSetupRequestEmailSender{
		host:      host,
		port:      cfg.Port,
		username:  strings.TrimSpace(cfg.Username),
		password:  strings.TrimSpace(cfg.Password),
		fromEmail: fromEmail,
		fromName:  strings.TrimSpace(cfg.FromName),
	}, nil
}

func (s *SMTPPaymentSetupRequestEmailSender) SendCustomerPaymentSetupRequest(input CustomerPaymentSetupRequestEmail) error {
	if s == nil {
		return fmt.Errorf("payment setup request email sender is required")
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
	subject := fmt.Sprintf("Complete your payment setup for %s", workspaceName)
	body := buildCustomerPaymentSetupRequestEmailBody(customerName, workspaceName, checkoutURL, input.RequestedByEmail, input.RequestKind)

	var message bytes.Buffer
	fmt.Fprintf(&message, "From: %s\r\n", formatEmailAddress(s.fromName, s.fromEmail))
	fmt.Fprintf(&message, "To: %s\r\n", toEmail)
	fmt.Fprintf(&message, "Subject: %s\r\n", subject)
	fmt.Fprintf(&message, "MIME-Version: 1.0\r\n")
	fmt.Fprintf(&message, "Content-Type: text/plain; charset=UTF-8\r\n")
	fmt.Fprintf(&message, "\r\n%s", body)

	addr := fmt.Sprintf("%s:%d", s.host, s.port)
	var auth smtp.Auth
	if s.username != "" {
		auth = smtp.PlainAuth("", s.username, s.password, s.host)
	}
	return smtp.SendMail(addr, auth, s.fromEmail, []string{toEmail}, message.Bytes())
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
