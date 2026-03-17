package service

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"

	"usage-billing-control-plane/internal/domain"
)

type OIDCBrowserSSOProviderConfig struct {
	Key          string
	DisplayName  string
	IssuerURL    string
	ClientID     string
	ClientSecret string
	Scopes       []string
}

type OIDCBrowserSSOProvider struct {
	key          string
	displayName  string
	clientID     string
	clientSecret string
	provider     *oidc.Provider
	verifier     *oidc.IDTokenVerifier
	scopes       []string
}

func NewOIDCBrowserSSOProvider(ctx context.Context, cfg OIDCBrowserSSOProviderConfig) (*OIDCBrowserSSOProvider, error) {
	key := normalizeBrowserSSOProviderKey(cfg.Key)
	if key == "" {
		return nil, fmt.Errorf("oidc provider key is required")
	}
	displayName := strings.TrimSpace(cfg.DisplayName)
	if displayName == "" {
		return nil, fmt.Errorf("oidc provider display name is required")
	}
	issuerURL := strings.TrimSpace(cfg.IssuerURL)
	clientID := strings.TrimSpace(cfg.ClientID)
	clientSecret := strings.TrimSpace(cfg.ClientSecret)
	if issuerURL == "" || clientID == "" || clientSecret == "" {
		return nil, fmt.Errorf("oidc provider issuer_url, client_id, and client_secret are required")
	}

	provider, err := oidc.NewProvider(ctx, issuerURL)
	if err != nil {
		return nil, fmt.Errorf("discover oidc provider %s: %w", key, err)
	}

	scopes := normalizeOIDCScopes(cfg.Scopes)
	return &OIDCBrowserSSOProvider{
		key:          key,
		displayName:  displayName,
		clientID:     clientID,
		clientSecret: clientSecret,
		provider:     provider,
		verifier:     provider.Verifier(&oidc.Config{ClientID: clientID}),
		scopes:       scopes,
	}, nil
}

func (p *OIDCBrowserSSOProvider) Definition() BrowserSSOProviderDefinition {
	return BrowserSSOProviderDefinition{
		Key:         p.key,
		DisplayName: p.displayName,
		Type:        domain.BrowserSSOProviderTypeOIDC,
	}
}

func (p *OIDCBrowserSSOProvider) BuildAuthCodeURL(state, nonce, codeChallenge, redirectURI string) (string, error) {
	redirectURI = strings.TrimSpace(redirectURI)
	if redirectURI == "" {
		return "", fmt.Errorf("redirect uri is required")
	}
	authURL := p.oauthConfig(redirectURI).AuthCodeURL(
		strings.TrimSpace(state),
		oauth2.AccessTypeOffline,
		oauth2.SetAuthURLParam("nonce", strings.TrimSpace(nonce)),
		oauth2.SetAuthURLParam("code_challenge", strings.TrimSpace(codeChallenge)),
		oauth2.SetAuthURLParam("code_challenge_method", "S256"),
	)
	return authURL, nil
}

func (p *OIDCBrowserSSOProvider) Exchange(ctx context.Context, redirectURI, code, codeVerifier, nonce string) (BrowserSSOClaims, error) {
	token, err := p.oauthConfig(redirectURI).Exchange(
		ctx,
		strings.TrimSpace(code),
		oauth2.SetAuthURLParam("code_verifier", strings.TrimSpace(codeVerifier)),
	)
	if err != nil {
		return BrowserSSOClaims{}, err
	}

	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok || strings.TrimSpace(rawIDToken) == "" {
		return BrowserSSOClaims{}, fmt.Errorf("oidc id_token is missing")
	}
	idToken, err := p.verifier.Verify(ctx, rawIDToken)
	if err != nil {
		return BrowserSSOClaims{}, fmt.Errorf("verify oidc id_token: %w", err)
	}

	var claims struct {
		Subject       string `json:"sub"`
		Email         string `json:"email"`
		EmailVerified bool   `json:"email_verified"`
		Name          string `json:"name"`
		Nonce         string `json:"nonce"`
	}
	if err := idToken.Claims(&claims); err != nil {
		return BrowserSSOClaims{}, fmt.Errorf("decode oidc claims: %w", err)
	}
	if nonce != "" && subtleStringMismatch(claims.Nonce, nonce) {
		return BrowserSSOClaims{}, fmt.Errorf("oidc nonce mismatch")
	}
	out := BrowserSSOClaims{
		Subject:       strings.TrimSpace(claims.Subject),
		Email:         strings.ToLower(strings.TrimSpace(claims.Email)),
		EmailVerified: claims.EmailVerified,
		DisplayName:   strings.TrimSpace(claims.Name),
	}

	if out.Email == "" || out.DisplayName == "" {
		userInfo, infoErr := p.provider.UserInfo(ctx, oauth2.StaticTokenSource(token))
		if infoErr == nil {
			var infoClaims struct {
				Subject       string `json:"sub"`
				Email         string `json:"email"`
				EmailVerified bool   `json:"email_verified"`
				Name          string `json:"name"`
			}
			if err := userInfo.Claims(&infoClaims); err == nil {
				if out.Subject == "" {
					out.Subject = strings.TrimSpace(infoClaims.Subject)
				}
				if out.Email == "" {
					out.Email = strings.ToLower(strings.TrimSpace(infoClaims.Email))
				}
				if !out.EmailVerified {
					out.EmailVerified = infoClaims.EmailVerified
				}
				if out.DisplayName == "" {
					out.DisplayName = strings.TrimSpace(infoClaims.Name)
				}
			}
		}
	}

	return out, nil
}

func BuildPKCECodeChallenge(verifier string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(verifier)))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

func (p *OIDCBrowserSSOProvider) oauthConfig(redirectURI string) *oauth2.Config {
	return &oauth2.Config{
		ClientID:     p.clientID,
		ClientSecret: p.clientSecret,
		Endpoint:     p.provider.Endpoint(),
		RedirectURL:  strings.TrimSpace(redirectURI),
		Scopes:       append([]string(nil), p.scopes...),
	}
}

func normalizeOIDCScopes(scopes []string) []string {
	if len(scopes) == 0 {
		return []string{oidc.ScopeOpenID, "profile", "email"}
	}
	seen := map[string]struct{}{}
	out := make([]string, 0, len(scopes)+1)
	for _, scope := range scopes {
		scope = strings.TrimSpace(scope)
		if scope == "" {
			continue
		}
		if _, ok := seen[scope]; ok {
			continue
		}
		seen[scope] = struct{}{}
		out = append(out, scope)
	}
	if _, ok := seen[oidc.ScopeOpenID]; !ok {
		out = append([]string{oidc.ScopeOpenID}, out...)
	}
	return out
}

func subtleStringMismatch(expected, provided string) bool {
	return strings.TrimSpace(expected) != strings.TrimSpace(provided)
}
