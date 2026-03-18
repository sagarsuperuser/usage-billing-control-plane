package service

import (
	"bytes"
	"fmt"
	"net/smtp"
	"strings"
	"time"
)

type PasswordResetEmail struct {
	ToEmail   string
	ResetURL  string
	ExpiresAt time.Time
}

type PasswordResetEmailSender interface {
	SendPasswordReset(input PasswordResetEmail) error
}

type SMTPPasswordResetEmailConfig struct {
	Host      string
	Port      int
	Username  string
	Password  string
	FromEmail string
	FromName  string
}

type SMTPPasswordResetEmailSender struct {
	host      string
	port      int
	username  string
	password  string
	fromEmail string
	fromName  string
}

func NewSMTPPasswordResetEmailSender(cfg SMTPPasswordResetEmailConfig) (*SMTPPasswordResetEmailSender, error) {
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
	return &SMTPPasswordResetEmailSender{
		host:      host,
		port:      cfg.Port,
		username:  strings.TrimSpace(cfg.Username),
		password:  strings.TrimSpace(cfg.Password),
		fromEmail: fromEmail,
		fromName:  strings.TrimSpace(cfg.FromName),
	}, nil
}

func (s *SMTPPasswordResetEmailSender) SendPasswordReset(input PasswordResetEmail) error {
	if s == nil {
		return fmt.Errorf("password reset email sender is required")
	}
	toEmail := strings.ToLower(strings.TrimSpace(input.ToEmail))
	if toEmail == "" {
		return fmt.Errorf("password reset recipient email is required")
	}
	resetURL := strings.TrimSpace(input.ResetURL)
	if resetURL == "" {
		return fmt.Errorf("password reset url is required")
	}
	subject := "Reset your Alpha password"
	body := buildPasswordResetEmailBody(resetURL, input.ExpiresAt)

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

func buildPasswordResetEmailBody(resetURL string, expiresAt time.Time) string {
	lines := []string{
		"We received a request to reset your Alpha password.",
		"",
		"Open this link to choose a new password:",
		resetURL,
	}
	if !expiresAt.IsZero() {
		lines = append(lines, "", fmt.Sprintf("Expires: %s", expiresAt.UTC().Format(time.RFC1123)))
	}
	lines = append(lines,
		"",
		"If you did not request a password reset, you can ignore this email.",
	)
	return strings.Join(lines, "\r\n")
}
