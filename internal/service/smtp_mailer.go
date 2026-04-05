package service

import (
	"bytes"
	"fmt"
	"net/smtp"
	"strings"
	"time"
)

// ---------------------------------------------------------------------------
// SMTPMailer — unified SMTP transport
//
// All email senders (password reset, workspace invitation, payment setup)
// delegate to a single SMTPMailer for the actual SMTP call. This eliminates
// duplicated host/port/auth configuration and provides a single place to
// add observability (duration, success/failure metrics).
// ---------------------------------------------------------------------------

// SMTPConfig holds the connection details shared across all email types.
type SMTPConfig struct {
	Host      string
	Port      int
	Username  string
	Password  string
	FromEmail string
	FromName  string
}

// SMTPMailer is the low-level transport that sends RFC 5322 messages via SMTP.
type SMTPMailer struct {
	host      string
	port      int
	username  string
	password  string
	fromEmail string
	fromName  string
}

// NewSMTPMailer validates config and returns a ready-to-use mailer.
func NewSMTPMailer(cfg SMTPConfig) (*SMTPMailer, error) {
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
	return &SMTPMailer{
		host:      host,
		port:      cfg.Port,
		username:  strings.TrimSpace(cfg.Username),
		password:  strings.TrimSpace(cfg.Password),
		fromEmail: fromEmail,
		fromName:  strings.TrimSpace(cfg.FromName),
	}, nil
}

// SMTPMessage is the envelope + content for a single outbound email.
type SMTPMessage struct {
	To      string
	Subject string
	Body    string // plain-text body
}

// SMTPSendResult contains timing metadata returned after a successful send.
type SMTPSendResult struct {
	Duration time.Duration
}

// Send dispatches a single plain-text email via SMTP.
// Returns timing metadata on success or an error on failure.
func (m *SMTPMailer) Send(msg SMTPMessage) (SMTPSendResult, error) {
	if m == nil {
		return SMTPSendResult{}, fmt.Errorf("smtp mailer is nil")
	}
	toEmail := strings.ToLower(strings.TrimSpace(msg.To))
	if toEmail == "" {
		return SMTPSendResult{}, fmt.Errorf("recipient email is required")
	}
	subject := strings.TrimSpace(msg.Subject)
	if subject == "" {
		return SMTPSendResult{}, fmt.Errorf("email subject is required")
	}

	var buf bytes.Buffer
	fmt.Fprintf(&buf, "From: %s\r\n", formatEmailAddress(m.fromName, m.fromEmail))
	fmt.Fprintf(&buf, "To: %s\r\n", toEmail)
	fmt.Fprintf(&buf, "Subject: %s\r\n", subject)
	fmt.Fprintf(&buf, "MIME-Version: 1.0\r\n")
	fmt.Fprintf(&buf, "Content-Type: text/plain; charset=UTF-8\r\n")
	fmt.Fprintf(&buf, "\r\n%s", msg.Body)

	addr := fmt.Sprintf("%s:%d", m.host, m.port)
	var auth smtp.Auth
	if m.username != "" {
		auth = smtp.PlainAuth("", m.username, m.password, m.host)
	}

	start := time.Now()
	err := smtp.SendMail(addr, auth, m.fromEmail, []string{toEmail}, buf.Bytes())
	dur := time.Since(start)

	if err != nil {
		return SMTPSendResult{Duration: dur}, fmt.Errorf("smtp send to %s failed after %s: %w", toEmail, dur.Round(time.Millisecond), err)
	}
	return SMTPSendResult{Duration: dur}, nil
}

// Host returns the configured SMTP host (for startup logging).
func (m *SMTPMailer) Host() string {
	if m == nil {
		return ""
	}
	return m.host
}

// FromEmail returns the configured sender address (for startup logging).
func (m *SMTPMailer) FromEmail() string {
	if m == nil {
		return ""
	}
	return m.fromEmail
}
