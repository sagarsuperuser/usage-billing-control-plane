package service

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"

	"usage-billing-control-plane/internal/domain"
	"usage-billing-control-plane/internal/store"
)

var (
	ErrPasswordResetTokenExpired = errors.New("password reset token expired")
	ErrPasswordResetTokenUsed    = errors.New("password reset token already used")
)

type passwordResetStore interface {
	GetUserByEmail(email string) (domain.User, error)
	GetUser(id string) (domain.User, error)
	GetUserPasswordCredential(userID string) (domain.UserPasswordCredential, error)
	UpsertUserPasswordCredential(input domain.UserPasswordCredential) (domain.UserPasswordCredential, error)
	CreatePasswordResetToken(input domain.PasswordResetToken) (domain.PasswordResetToken, error)
	GetPasswordResetTokenByTokenHash(tokenHash string) (domain.PasswordResetToken, error)
	UpdatePasswordResetToken(input domain.PasswordResetToken) (domain.PasswordResetToken, error)
}

type PasswordResetIssueResult struct {
	Token     domain.PasswordResetToken `json:"token"`
	RawToken  string                    `json:"raw_token"`
	UserEmail string                    `json:"user_email"`
}

type PasswordResetService struct {
	store passwordResetStore
	ttl   time.Duration
}

func NewPasswordResetService(repo passwordResetStore, ttl time.Duration) *PasswordResetService {
	if ttl <= 0 {
		ttl = time.Hour
	}
	return &PasswordResetService{store: repo, ttl: ttl}
}

func (s *PasswordResetService) IssuePasswordReset(email string) (PasswordResetIssueResult, error) {
	if s == nil || s.store == nil {
		return PasswordResetIssueResult{}, fmt.Errorf("%w: password reset repository is required", ErrValidation)
	}
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" {
		return PasswordResetIssueResult{}, fmt.Errorf("%w: email is required", ErrValidation)
	}
	user, err := s.store.GetUserByEmail(email)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return PasswordResetIssueResult{}, store.ErrNotFound
		}
		return PasswordResetIssueResult{}, err
	}
	if user.Status != domain.UserStatusActive {
		return PasswordResetIssueResult{}, store.ErrNotFound
	}
	if _, err := s.store.GetUserPasswordCredential(user.ID); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return PasswordResetIssueResult{}, store.ErrNotFound
		}
		return PasswordResetIssueResult{}, err
	}
	rawToken, tokenHash, err := newPasswordResetToken()
	if err != nil {
		return PasswordResetIssueResult{}, err
	}
	now := time.Now().UTC()
	resetToken, err := s.store.CreatePasswordResetToken(domain.PasswordResetToken{
		UserID:    user.ID,
		TokenHash: tokenHash,
		ExpiresAt: now.Add(s.ttl),
		CreatedAt: now,
		UpdatedAt: now,
	})
	if err != nil {
		return PasswordResetIssueResult{}, err
	}
	return PasswordResetIssueResult{
		Token:     resetToken,
		RawToken:  rawToken,
		UserEmail: user.Email,
	}, nil
}

func (s *PasswordResetService) ResetPassword(token, password string) (domain.User, error) {
	if s == nil || s.store == nil {
		return domain.User{}, fmt.Errorf("%w: password reset repository is required", ErrValidation)
	}
	tokenHash := hashPasswordResetToken(token)
	if tokenHash == "" {
		return domain.User{}, store.ErrNotFound
	}
	resetToken, err := s.store.GetPasswordResetTokenByTokenHash(tokenHash)
	if err != nil {
		return domain.User{}, err
	}
	now := time.Now().UTC()
	if resetToken.UsedAt != nil {
		return domain.User{}, ErrPasswordResetTokenUsed
	}
	if resetToken.ExpiresAt.Before(now) {
		return domain.User{}, ErrPasswordResetTokenExpired
	}
	user, err := s.store.GetUser(resetToken.UserID)
	if err != nil {
		return domain.User{}, err
	}
	hash, err := HashPassword(password)
	if err != nil {
		return domain.User{}, err
	}
	if _, err := s.store.UpsertUserPasswordCredential(domain.UserPasswordCredential{
		UserID:            user.ID,
		PasswordHash:      hash,
		PasswordUpdatedAt: now,
		UpdatedAt:         now,
	}); err != nil {
		return domain.User{}, err
	}
	resetToken.UsedAt = &now
	resetToken.UpdatedAt = now
	if _, err := s.store.UpdatePasswordResetToken(resetToken); err != nil {
		return domain.User{}, err
	}
	return user, nil
}

func newPasswordResetToken() (string, string, error) {
	var raw [32]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", "", fmt.Errorf("generate password reset token: %w", err)
	}
	token := base64.RawURLEncoding.EncodeToString(raw[:])
	return token, hashPasswordResetToken(token), nil
}

func hashPasswordResetToken(token string) string {
	token = strings.TrimSpace(token)
	if token == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(token))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}
