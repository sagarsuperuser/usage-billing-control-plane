package service

import (
	"bytes"
	"fmt"
	"net/smtp"
	"strings"
	"time"
)

type WorkspaceInvitationEmail struct {
	ToEmail        string
	WorkspaceName  string
	Role           string
	AcceptURL      string
	ExpiresAt      time.Time
	InvitedByEmail string
}

type WorkspaceInvitationEmailSender interface {
	SendWorkspaceInvitation(input WorkspaceInvitationEmail) error
}

type SMTPWorkspaceInvitationEmailConfig struct {
	Host      string
	Port      int
	Username  string
	Password  string
	FromEmail string
	FromName  string
}

type SMTPWorkspaceInvitationEmailSender struct {
	host      string
	port      int
	username  string
	password  string
	fromEmail string
	fromName  string
}

func NewSMTPWorkspaceInvitationEmailSender(cfg SMTPWorkspaceInvitationEmailConfig) (*SMTPWorkspaceInvitationEmailSender, error) {
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
	return &SMTPWorkspaceInvitationEmailSender{
		host:      host,
		port:      cfg.Port,
		username:  strings.TrimSpace(cfg.Username),
		password:  strings.TrimSpace(cfg.Password),
		fromEmail: fromEmail,
		fromName:  strings.TrimSpace(cfg.FromName),
	}, nil
}

func (s *SMTPWorkspaceInvitationEmailSender) SendWorkspaceInvitation(input WorkspaceInvitationEmail) error {
	if s == nil {
		return fmt.Errorf("workspace invitation email sender is required")
	}
	toEmail := strings.ToLower(strings.TrimSpace(input.ToEmail))
	if toEmail == "" {
		return fmt.Errorf("invite recipient email is required")
	}
	acceptURL := strings.TrimSpace(input.AcceptURL)
	if acceptURL == "" {
		return fmt.Errorf("invite accept url is required")
	}
	workspaceName := strings.TrimSpace(input.WorkspaceName)
	if workspaceName == "" {
		workspaceName = "your workspace"
	}
	role := strings.TrimSpace(input.Role)
	if role == "" {
		role = "member"
	}
	subject := fmt.Sprintf("Join %s in Alpha", workspaceName)
	body := buildWorkspaceInvitationEmailBody(workspaceName, role, acceptURL, input.ExpiresAt, input.InvitedByEmail)

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

func buildWorkspaceInvitationEmailBody(workspaceName, role, acceptURL string, expiresAt time.Time, invitedByEmail string) string {
	lines := []string{
		fmt.Sprintf("You have been invited to join %s in Alpha.", workspaceName),
		"",
		fmt.Sprintf("Role: %s", role),
	}
	if invitedBy := strings.TrimSpace(invitedByEmail); invitedBy != "" {
		lines = append(lines, fmt.Sprintf("Invited by: %s", invitedBy))
	}
	if !expiresAt.IsZero() {
		lines = append(lines, fmt.Sprintf("Expires: %s", expiresAt.UTC().Format(time.RFC1123)))
	}
	lines = append(lines,
		"",
		"Open this link to review and accept the invitation:",
		acceptURL,
		"",
		"If you were not expecting this invitation, you can ignore this email.",
	)
	return strings.Join(lines, "\r\n")
}

func formatEmailAddress(name, email string) string {
	name = strings.TrimSpace(name)
	email = strings.TrimSpace(email)
	if name == "" {
		return email
	}
	return fmt.Sprintf("%s <%s>", name, email)
}
