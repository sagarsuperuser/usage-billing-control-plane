package service

import (
	"fmt"
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

// SMTPWorkspaceInvitationEmailSender sends workspace invitation emails via a shared SMTPMailer.
type SMTPWorkspaceInvitationEmailSender struct {
	mailer *SMTPMailer
}

func NewSMTPWorkspaceInvitationEmailSender(mailer *SMTPMailer) *SMTPWorkspaceInvitationEmailSender {
	return &SMTPWorkspaceInvitationEmailSender{mailer: mailer}
}

func (s *SMTPWorkspaceInvitationEmailSender) SendWorkspaceInvitation(input WorkspaceInvitationEmail) error {
	if s == nil || s.mailer == nil {
		return fmt.Errorf("workspace invitation email sender is not configured")
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
	_, err := s.mailer.Send(SMTPMessage{
		To:      toEmail,
		Subject: fmt.Sprintf("Join %s in Alpha", workspaceName),
		Body:    buildWorkspaceInvitationEmailBody(workspaceName, role, acceptURL, input.ExpiresAt, input.InvitedByEmail),
	})
	return err
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
