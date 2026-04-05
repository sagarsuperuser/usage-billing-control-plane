package service

import (
	"fmt"
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

// SMTPPasswordResetEmailSender sends password reset emails via a shared SMTPMailer.
type SMTPPasswordResetEmailSender struct {
	mailer *SMTPMailer
}

func NewSMTPPasswordResetEmailSender(mailer *SMTPMailer) *SMTPPasswordResetEmailSender {
	return &SMTPPasswordResetEmailSender{mailer: mailer}
}

func (s *SMTPPasswordResetEmailSender) SendPasswordReset(input PasswordResetEmail) error {
	if s == nil || s.mailer == nil {
		return fmt.Errorf("password reset email sender is not configured")
	}
	toEmail := strings.ToLower(strings.TrimSpace(input.ToEmail))
	if toEmail == "" {
		return fmt.Errorf("password reset recipient email is required")
	}
	resetURL := strings.TrimSpace(input.ResetURL)
	if resetURL == "" {
		return fmt.Errorf("password reset url is required")
	}
	_, err := s.mailer.Send(SMTPMessage{
		To:      toEmail,
		Subject: "Reset your Alpha password",
		Body:    buildPasswordResetEmailBody(resetURL, input.ExpiresAt),
	})
	return err
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
