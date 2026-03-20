package api_test

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/alexedwards/scs/postgresstore"
	"github.com/alexedwards/scs/v2"
	_ "github.com/jackc/pgx/v5/stdlib"

	"usage-billing-control-plane/internal/api"
	"usage-billing-control-plane/internal/domain"
	"usage-billing-control-plane/internal/store"
)

func TestWorkspaceAccessSubresources(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is required for integration tests")
	}

	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	repo := store.NewPostgresStore(db)
	if err := repo.Migrate(); err != nil {
		t.Fatalf("migrate db: %v", err)
	}
	resetTables(t, db)
	mustCreatePlatformAPIKey(t, repo, "platform-admin")

	now := time.Now().UTC()
	if _, err := repo.CreateTenant(domain.Tenant{
		ID:        "tenant_access",
		Name:      "Tenant Access",
		Status:    domain.TenantStatusActive,
		CreatedAt: now,
		UpdatedAt: now,
	}); err != nil {
		t.Fatalf("create tenant: %v", err)
	}
	user, err := repo.CreateUser(domain.User{
		ID:          "usr_workspace_admin",
		Email:       "owner@tenant.test",
		DisplayName: "Owner",
		Status:      domain.UserStatusActive,
		CreatedAt:   now,
		UpdatedAt:   now,
	})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	if _, err := repo.UpsertUserTenantMembership(domain.UserTenantMembership{
		UserID:    user.ID,
		TenantID:  "tenant_access",
		Role:      "admin",
		Status:    domain.UserTenantMembershipStatusActive,
		CreatedAt: now,
		UpdatedAt: now,
	}); err != nil {
		t.Fatalf("create membership: %v", err)
	}
	backupAdmin, err := repo.CreateUser(domain.User{
		ID:          "usr_workspace_backup_admin",
		Email:       "backup-admin@tenant.test",
		DisplayName: "Backup Admin",
		Status:      domain.UserStatusActive,
		CreatedAt:   now,
		UpdatedAt:   now,
	})
	if err != nil {
		t.Fatalf("create backup admin: %v", err)
	}
	if _, err := repo.UpsertUserTenantMembership(domain.UserTenantMembership{
		UserID:    backupAdmin.ID,
		TenantID:  "tenant_access",
		Role:      "admin",
		Status:    domain.UserTenantMembershipStatusActive,
		CreatedAt: now,
		UpdatedAt: now,
	}); err != nil {
		t.Fatalf("create backup admin membership: %v", err)
	}

	authorizer, err := api.NewDBAPIKeyAuthorizer(repo)
	if err != nil {
		t.Fatalf("new authorizer: %v", err)
	}

	ts := httptest.NewServer(api.NewServer(repo, api.WithAPIKeyAuthorizer(authorizer)).Handler())
	defer ts.Close()

	members := getJSON(t, ts.URL+"/internal/tenants/tenant_access/members", "platform-admin", http.StatusOK)
	memberItems := members["items"].([]any)
	if len(memberItems) != 2 {
		t.Fatalf("expected 2 workspace members, got %d", len(memberItems))
	}
	foundPrimaryMember := false
	for _, raw := range memberItems {
		member := raw.(map[string]any)
		if member["email"] == user.Email {
			foundPrimaryMember = true
			break
		}
	}
	if !foundPrimaryMember {
		t.Fatalf("expected member email %q in workspace members", user.Email)
	}

	inviteResult := postJSON(t, ts.URL+"/internal/tenants/tenant_access/invitations", map[string]any{
		"email": "writer@tenant.test",
		"role":  "writer",
	}, "platform-admin", http.StatusCreated)
	invite := inviteResult["invitation"].(map[string]any)
	inviteID, _ := invite["id"].(string)
	if inviteID == "" {
		t.Fatalf("expected invitation id")
	}
	if invite["status"] != string(domain.WorkspaceInvitationStatusPending) {
		t.Fatalf("expected pending invitation, got %#v", invite["status"])
	}

	invitations := getJSON(t, ts.URL+"/internal/tenants/tenant_access/invitations", "platform-admin", http.StatusOK)
	inviteItems := invitations["items"].([]any)
	if len(inviteItems) != 1 {
		t.Fatalf("expected 1 invitation, got %d", len(inviteItems))
	}

	updatedMember := patchJSON(t, ts.URL+"/internal/tenants/tenant_access/members/"+user.ID, map[string]any{
		"role":   "writer",
		"reason": "support override",
	}, "platform-admin", http.StatusOK)
	memberPayload := updatedMember["member"].(map[string]any)
	if memberPayload["role"] != "writer" {
		t.Fatalf("expected updated member role writer, got %#v", memberPayload["role"])
	}

	revokeResult := postJSON(t, ts.URL+"/internal/tenants/tenant_access/invitations/"+inviteID+"/revoke", map[string]any{
		"reason": "support cleanup",
	}, "platform-admin", http.StatusOK)
	revoked := revokeResult["invitation"].(map[string]any)
	if revoked["status"] != string(domain.WorkspaceInvitationStatusRevoked) {
		t.Fatalf("expected revoked invitation, got %#v", revoked["status"])
	}

	deleteRequest, err := http.NewRequest(http.MethodDelete, ts.URL+"/internal/tenants/tenant_access/members/"+user.ID+"?reason="+url.QueryEscape("suspend access"), nil)
	if err != nil {
		t.Fatalf("build delete request: %v", err)
	}
	deleteRequest.Header.Set("X-API-Key", "platform-admin")
	deleteResp, err := http.DefaultClient.Do(deleteRequest)
	if err != nil {
		t.Fatalf("delete member request: %v", err)
	}
	defer deleteResp.Body.Close()
	if deleteResp.StatusCode != http.StatusNoContent {
		t.Fatalf("expected 204 on delete, got %d", deleteResp.StatusCode)
	}

	membership, err := repo.GetUserTenantMembership(user.ID, "tenant_access")
	if err != nil {
		t.Fatalf("get membership after remove: %v", err)
	}
	if membership.Status != domain.UserTenantMembershipStatusDisabled {
		t.Fatalf("expected disabled membership after remove, got %q", membership.Status)
	}

	auditPage, err := repo.ListTenantAuditEvents(store.TenantAuditFilter{TenantID: "tenant_access", Limit: 20})
	if err != nil {
		t.Fatalf("list tenant audit events: %v", err)
	}
	foundRoleChange := false
	foundInviteRevoke := false
	foundDisable := false
	for _, item := range auditPage.Items {
		switch item.Action {
		case "workspace_member_role_changed":
			foundRoleChange = true
			if got := item.Metadata["reason"]; got != "support override" {
				t.Fatalf("expected role change reason support override, got %#v", got)
			}
		case "workspace_invitation_revoked":
			foundInviteRevoke = true
			if got := item.Metadata["reason"]; got != "support cleanup" {
				t.Fatalf("expected invite revoke reason support cleanup, got %#v", got)
			}
		case "workspace_member_disabled":
			foundDisable = true
			if got := item.Metadata["reason"]; got != "suspend access" {
				t.Fatalf("expected disable reason suspend access, got %#v", got)
			}
		}
	}
	if !foundRoleChange {
		t.Fatalf("expected workspace_member_role_changed tenant audit event")
	}
	if !foundInviteRevoke {
		t.Fatalf("expected workspace_invitation_revoked tenant audit event")
	}
	if !foundDisable {
		t.Fatalf("expected workspace_member_disabled tenant audit event")
	}
}

func TestWorkspaceInvitationPreviewAndAcceptFlow(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is required for integration tests")
	}

	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	repo := store.NewPostgresStore(db)
	if err := repo.Migrate(); err != nil {
		t.Fatalf("migrate db: %v", err)
	}
	resetTables(t, db)
	if _, err := db.Exec(`TRUNCATE TABLE sessions`); err != nil {
		t.Fatalf("truncate sessions: %v", err)
	}
	mustCreatePlatformAPIKey(t, repo, "platform-admin")

	now := time.Now().UTC()
	if _, err := repo.CreateTenant(domain.Tenant{
		ID:        "tenant_accept",
		Name:      "Tenant Accept",
		Status:    domain.TenantStatusActive,
		CreatedAt: now,
		UpdatedAt: now,
	}); err != nil {
		t.Fatalf("create tenant: %v", err)
	}
	mustCreateBrowserUser(t, repo, browserUserFixture{
		email:    "invitee@tenant.test",
		password: "invitee password 123",
	})

	sessionManager := scs.New()
	sessionManager.Store = postgresstore.New(db)
	sessionManager.Lifetime = 12 * time.Hour
	sessionManager.Cookie.Name = "test_ui_invite_session"
	sessionManager.Cookie.HttpOnly = true
	sessionManager.Cookie.Secure = false
	sessionManager.Cookie.SameSite = http.SameSiteLaxMode

	ts := httptest.NewServer(api.NewServer(
		repo,
		api.WithAPIKeyAuthorizer(mustNewDBAuthorizer(t, repo)),
		api.WithSessionManager(sessionManager),
		api.WithUIPublicBaseURL("https://app.example.com"),
	).Handler())
	defer ts.Close()

	inviteResult := postJSON(t, ts.URL+"/internal/tenants/tenant_accept/invitations", map[string]any{
		"email": "invitee@tenant.test",
		"role":  "writer",
	}, "platform-admin", http.StatusCreated)
	acceptPath, _ := inviteResult["accept_path"].(string)
	if acceptPath == "" {
		t.Fatalf("expected accept_path in invitation response")
	}
	token := strings.TrimPrefix(acceptPath, "/invite/")
	previewResp, err := http.Get(ts.URL + "/v1/ui/invitations/" + url.PathEscape(token))
	if err != nil {
		t.Fatalf("preview request: %v", err)
	}
	defer previewResp.Body.Close()
	if previewResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 on preview, got %d", previewResp.StatusCode)
	}

	client := newSessionClient(t)
	loginResp := sessionPostJSON(t, client, ts.URL+"/v1/ui/sessions/login", map[string]any{
		"email":    "invitee@tenant.test",
		"password": "invitee password 123",
	}, "", http.StatusCreated)
	csrfToken, _ := loginResp["csrf_token"].(string)
	acceptResp := sessionPostJSON(t, client, ts.URL+"/v1/ui/invitations/"+url.PathEscape(token)+"/accept", map[string]any{}, csrfToken, http.StatusCreated)
	sessionPayload := acceptResp["session"].(map[string]any)
	if got, _ := sessionPayload["tenant_id"].(string); got != "tenant_accept" {
		t.Fatalf("expected tenant_accept session after invite acceptance, got %q", got)
	}
	membership, err := repo.GetUserTenantMembership(strings.TrimSpace(loginResp["subject_id"].(string)), "tenant_accept")
	if err != nil {
		t.Fatalf("load membership after accept: %v", err)
	}
	if membership.Status != domain.UserTenantMembershipStatusActive || membership.Role != "writer" {
		t.Fatalf("expected active writer membership after invite accept, got status=%q role=%q", membership.Status, membership.Role)
	}
}

func TestWorkspaceInvitationRegisterFlow(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is required for integration tests")
	}

	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	repo := store.NewPostgresStore(db)
	if err := repo.Migrate(); err != nil {
		t.Fatalf("migrate db: %v", err)
	}
	resetTables(t, db)
	if _, err := db.Exec(`TRUNCATE TABLE sessions`); err != nil {
		t.Fatalf("truncate sessions: %v", err)
	}
	mustCreatePlatformAPIKey(t, repo, "platform-admin")

	now := time.Now().UTC()
	if _, err := repo.CreateTenant(domain.Tenant{
		ID:        "tenant_register",
		Name:      "Tenant Register",
		Status:    domain.TenantStatusActive,
		CreatedAt: now,
		UpdatedAt: now,
	}); err != nil {
		t.Fatalf("create tenant: %v", err)
	}

	sessionManager := scs.New()
	sessionManager.Store = postgresstore.New(db)
	sessionManager.Lifetime = 12 * time.Hour
	sessionManager.Cookie.Name = "test_ui_invite_register_session"
	sessionManager.Cookie.HttpOnly = true
	sessionManager.Cookie.Secure = false
	sessionManager.Cookie.SameSite = http.SameSiteLaxMode

	ts := httptest.NewServer(api.NewServer(
		repo,
		api.WithAPIKeyAuthorizer(mustNewDBAuthorizer(t, repo)),
		api.WithSessionManager(sessionManager),
		api.WithUIPublicBaseURL("https://app.example.com"),
	).Handler())
	defer ts.Close()

	inviteResult := postJSON(t, ts.URL+"/internal/tenants/tenant_register/invitations", map[string]any{
		"email": "new-invitee@tenant.test",
		"role":  "admin",
	}, "platform-admin", http.StatusCreated)
	acceptPath, _ := inviteResult["accept_path"].(string)
	if acceptPath == "" {
		t.Fatalf("expected accept_path in invitation response")
	}
	token := strings.TrimPrefix(acceptPath, "/invite/")

	preview := getJSON(t, ts.URL+"/v1/ui/invitations/"+url.PathEscape(token), "", http.StatusOK)
	if accountExists, _ := preview["account_exists"].(bool); accountExists {
		t.Fatalf("expected no pre-existing account for invite")
	}

	client := newSessionClient(t)
	registerResp := sessionPostJSON(t, client, ts.URL+"/v1/ui/invitations/"+url.PathEscape(token)+"/register", map[string]any{
		"display_name": "New Invitee",
		"password":     "new invitee password 123",
	}, "", http.StatusCreated)
	sessionPayload := registerResp["session"].(map[string]any)
	if got, _ := sessionPayload["tenant_id"].(string); got != "tenant_register" {
		t.Fatalf("expected tenant_register session after invite registration, got %q", got)
	}

	user, err := repo.GetUserByEmail("new-invitee@tenant.test")
	if err != nil {
		t.Fatalf("load registered user: %v", err)
	}
	if user.DisplayName != "New Invitee" {
		t.Fatalf("expected registered display name, got %q", user.DisplayName)
	}
	if _, err := repo.GetUserPasswordCredential(user.ID); err != nil {
		t.Fatalf("expected password credential for registered invitee: %v", err)
	}
	membership, err := repo.GetUserTenantMembership(user.ID, "tenant_register")
	if err != nil {
		t.Fatalf("load membership after register: %v", err)
	}
	if membership.Status != domain.UserTenantMembershipStatusActive || membership.Role != "admin" {
		t.Fatalf("expected active admin membership after invite register, got status=%q role=%q", membership.Status, membership.Role)
	}
}

func TestTenantWorkspaceServiceAccountLifecycle(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is required for integration tests")
	}

	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	repo := store.NewPostgresStore(db)
	if err := repo.Migrate(); err != nil {
		t.Fatalf("migrate db: %v", err)
	}
	resetTables(t, db)
	if _, err := db.Exec(`TRUNCATE TABLE sessions`); err != nil {
		t.Fatalf("truncate sessions: %v", err)
	}

	now := time.Now().UTC()
	if _, err := repo.CreateTenant(domain.Tenant{
		ID:        "tenant_service_accounts",
		Name:      "Tenant Service Accounts",
		Status:    domain.TenantStatusActive,
		CreatedAt: now,
		UpdatedAt: now,
	}); err != nil {
		t.Fatalf("create tenant: %v", err)
	}
	mustCreateBrowserUser(t, repo, browserUserFixture{
		email:    "admin@tenant.test",
		password: "tenant admin password 123",
	})
	user, err := repo.GetUserByEmail("admin@tenant.test")
	if err != nil {
		t.Fatalf("get user: %v", err)
	}
	if _, err := repo.UpsertUserTenantMembership(domain.UserTenantMembership{
		UserID:    user.ID,
		TenantID:  "tenant_service_accounts",
		Role:      "admin",
		Status:    domain.UserTenantMembershipStatusActive,
		CreatedAt: now,
		UpdatedAt: now,
	}); err != nil {
		t.Fatalf("create membership: %v", err)
	}

	sessionManager := scs.New()
	sessionManager.Store = postgresstore.New(db)
	sessionManager.Lifetime = 12 * time.Hour
	sessionManager.Cookie.Name = "test_ui_service_account_session"
	sessionManager.Cookie.HttpOnly = true
	sessionManager.Cookie.Secure = false
	sessionManager.Cookie.SameSite = http.SameSiteLaxMode

	ts := httptest.NewServer(api.NewServer(
		repo,
		api.WithAPIKeyAuthorizer(mustNewDBAuthorizer(t, repo)),
		api.WithSessionManager(sessionManager),
	).Handler())
	defer ts.Close()

	client := newSessionClient(t)
	loginResp := sessionPostJSON(t, client, ts.URL+"/v1/ui/sessions/login", map[string]any{
		"email":     "admin@tenant.test",
		"password":  "tenant admin password 123",
		"tenant_id": "tenant_service_accounts",
	}, "", http.StatusCreated)
	csrfToken, _ := loginResp["csrf_token"].(string)

	created := sessionPostJSON(t, client, ts.URL+"/v1/workspace/service-accounts", map[string]any{
		"name":                     "Acme ERP Sync",
		"role":                     "writer",
		"description":              "ERP integration worker",
		"purpose":                  "erp-sync",
		"environment":              "prod",
		"issue_initial_credential": true,
	}, csrfToken, http.StatusCreated)
	serviceAccount := created["service_account"].(map[string]any)
	serviceAccountID, _ := serviceAccount["id"].(string)
	if serviceAccountID == "" {
		t.Fatalf("expected service account id")
	}
	credential := created["credential"].(map[string]any)
	credentialID, _ := credential["id"].(string)
	if credential["owner_type"] != "service_account" {
		t.Fatalf("expected service_account owner type, got %#v", credential["owner_type"])
	}
	if credential["owner_id"] != serviceAccountID {
		t.Fatalf("expected owner_id %q, got %#v", serviceAccountID, credential["owner_id"])
	}
	createdSecret, _ := created["secret"].(string)
	if createdSecret == "" {
		t.Fatalf("expected initial secret")
	}

	listed := sessionGetJSON(t, client, ts.URL+"/v1/workspace/service-accounts", http.StatusOK)
	items := listed["items"].([]any)
	if len(items) != 1 {
		t.Fatalf("expected 1 service account, got %d", len(items))
	}
	listedAccount := items[0].(map[string]any)
	if listedAccount["active_credential_count"].(float64) < 1 {
		t.Fatalf("expected active credential count")
	}
	if got, _ := listedAccount["status"].(string); got != domain.ServiceAccountStatusActive {
		t.Fatalf("expected active service account status, got %q", got)
	}
	_ = sessionGetJSON(t, client, ts.URL+"/v1/workspace/service-accounts/"+url.PathEscape(serviceAccountID)+"/audit?limit=10", http.StatusOK)
	_ = getJSONArray(t, ts.URL+"/v1/customers", createdSecret, http.StatusOK)

	disabled := sessionPatchJSON(t, client, ts.URL+"/v1/workspace/service-accounts/"+url.PathEscape(serviceAccountID), map[string]any{
		"status": domain.ServiceAccountStatusDisabled,
	}, csrfToken, http.StatusOK)
	disabledAccount := disabled["service_account"].(map[string]any)
	if got, _ := disabledAccount["status"].(string); got != domain.ServiceAccountStatusDisabled {
		t.Fatalf("expected disabled service account status, got %q", got)
	}
	_ = getJSON(t, ts.URL+"/v1/customers", createdSecret, http.StatusUnauthorized)

	enabled := sessionPatchJSON(t, client, ts.URL+"/v1/workspace/service-accounts/"+url.PathEscape(serviceAccountID), map[string]any{
		"status": domain.ServiceAccountStatusActive,
	}, csrfToken, http.StatusOK)
	enabledAccount := enabled["service_account"].(map[string]any)
	if got, _ := enabledAccount["status"].(string); got != domain.ServiceAccountStatusActive {
		t.Fatalf("expected re-enabled service account status, got %q", got)
	}
	_ = getJSONArray(t, ts.URL+"/v1/customers", createdSecret, http.StatusOK)

	rotated := sessionPostJSON(t, client, ts.URL+"/v1/workspace/service-accounts/"+url.PathEscape(serviceAccountID)+"/credentials/"+url.PathEscape(credentialID)+"/rotate", map[string]any{}, csrfToken, http.StatusOK)
	rotatedCredential := rotated["credential"].(map[string]any)
	rotatedID, _ := rotatedCredential["id"].(string)
	if rotatedID == "" || rotatedID == credentialID {
		t.Fatalf("expected new rotated credential id, got %q", rotatedID)
	}
	if secret, _ := rotated["secret"].(string); secret == "" {
		t.Fatalf("expected rotated secret")
	}

	revokeResp := sessionPostJSON(t, client, ts.URL+"/v1/workspace/service-accounts/"+url.PathEscape(serviceAccountID)+"/credentials/"+url.PathEscape(rotatedID)+"/revoke", map[string]any{}, csrfToken, http.StatusOK)
	revokedCredential := revokeResp["credential"].(map[string]any)
	if revokedCredential["revoked_at"] == nil {
		t.Fatalf("expected revoked_at on revoked credential")
	}

	auditResp := sessionGetJSON(t, client, ts.URL+"/v1/workspace/service-accounts/"+url.PathEscape(serviceAccountID)+"/audit?limit=10", http.StatusOK)
	auditItems := auditResp["items"].([]any)
	if len(auditItems) < 3 {
		t.Fatalf("expected at least 3 audit events for service account lifecycle, got %d", len(auditItems))
	}
	firstEvent := auditItems[0].(map[string]any)
	metadata, ok := firstEvent["metadata"].(map[string]any)
	if !ok {
		t.Fatalf("expected metadata on service account audit event")
	}
	if metadata["owner_id"] != serviceAccountID {
		t.Fatalf("expected audit metadata owner_id %q, got %#v", serviceAccountID, metadata["owner_id"])
	}
}

func TestWorkspaceAccessBlocksLastActiveAdminPlatformOverride(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is required for integration tests")
	}

	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	repo := store.NewPostgresStore(db)
	if err := repo.Migrate(); err != nil {
		t.Fatalf("migrate db: %v", err)
	}
	resetTables(t, db)
	mustCreatePlatformAPIKey(t, repo, "platform-admin")

	now := time.Now().UTC()
	if _, err := repo.CreateTenant(domain.Tenant{
		ID:        "tenant_last_admin",
		Name:      "Tenant Last Admin",
		Status:    domain.TenantStatusActive,
		CreatedAt: now,
		UpdatedAt: now,
	}); err != nil {
		t.Fatalf("create tenant: %v", err)
	}
	user, err := repo.CreateUser(domain.User{
		ID:          "usr_last_admin",
		Email:       "last-admin@tenant.test",
		DisplayName: "Last Admin",
		Status:      domain.UserStatusActive,
		CreatedAt:   now,
		UpdatedAt:   now,
	})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	if _, err := repo.UpsertUserTenantMembership(domain.UserTenantMembership{
		UserID:    user.ID,
		TenantID:  "tenant_last_admin",
		Role:      "admin",
		Status:    domain.UserTenantMembershipStatusActive,
		CreatedAt: now,
		UpdatedAt: now,
	}); err != nil {
		t.Fatalf("create membership: %v", err)
	}

	ts := httptest.NewServer(api.NewServer(repo, api.WithAPIKeyAuthorizer(mustNewDBAuthorizer(t, repo))).Handler())
	defer ts.Close()

	roleResp := patchJSON(t, ts.URL+"/internal/tenants/tenant_last_admin/members/"+user.ID, map[string]any{
		"role":   "writer",
		"reason": "override test",
	}, "platform-admin", http.StatusConflict)
	if got, _ := roleResp["error_code"].(string); got != "last_active_admin_conflict" {
		t.Fatalf("expected last_active_admin_conflict, got %#v", roleResp["error_code"])
	}

	deleteReq, err := http.NewRequest(http.MethodDelete, ts.URL+"/internal/tenants/tenant_last_admin/members/"+user.ID+"?reason="+url.QueryEscape("override test"), nil)
	if err != nil {
		t.Fatalf("build delete request: %v", err)
	}
	deleteReq.Header.Set("X-API-Key", "platform-admin")
	deleteResp, err := http.DefaultClient.Do(deleteReq)
	if err != nil {
		t.Fatalf("delete member request: %v", err)
	}
	defer deleteResp.Body.Close()
	if deleteResp.StatusCode != http.StatusConflict {
		t.Fatalf("expected 409 on delete, got %d", deleteResp.StatusCode)
	}
	deletePayload := decodeResponseMap(t, deleteResp)
	if got, _ := deletePayload["error_code"].(string); got != "last_active_admin_conflict" {
		t.Fatalf("expected last_active_admin_conflict on delete, got %#v", deletePayload["error_code"])
	}
}

func TestTenantWorkspaceMembersRejectSelfMutation(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is required for integration tests")
	}

	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	repo := store.NewPostgresStore(db)
	if err := repo.Migrate(); err != nil {
		t.Fatalf("migrate db: %v", err)
	}
	resetTables(t, db)
	if _, err := db.Exec(`TRUNCATE TABLE sessions`); err != nil {
		t.Fatalf("truncate sessions: %v", err)
	}

	now := time.Now().UTC()
	if _, err := repo.CreateTenant(domain.Tenant{
		ID:        "tenant_self_guard",
		Name:      "Tenant Self Guard",
		Status:    domain.TenantStatusActive,
		CreatedAt: now,
		UpdatedAt: now,
	}); err != nil {
		t.Fatalf("create tenant: %v", err)
	}
	mustCreateBrowserUser(t, repo, browserUserFixture{
		email:    "self-admin@tenant.test",
		password: "tenant self guard password 123",
	})
	user, err := repo.GetUserByEmail("self-admin@tenant.test")
	if err != nil {
		t.Fatalf("get user: %v", err)
	}
	if _, err := repo.UpsertUserTenantMembership(domain.UserTenantMembership{
		UserID:    user.ID,
		TenantID:  "tenant_self_guard",
		Role:      "admin",
		Status:    domain.UserTenantMembershipStatusActive,
		CreatedAt: now,
		UpdatedAt: now,
	}); err != nil {
		t.Fatalf("create membership: %v", err)
	}

	sessionManager := scs.New()
	sessionManager.Store = postgresstore.New(db)
	sessionManager.Lifetime = 12 * time.Hour
	sessionManager.Cookie.Name = "test_ui_self_guard_session"
	sessionManager.Cookie.HttpOnly = true
	sessionManager.Cookie.Secure = false
	sessionManager.Cookie.SameSite = http.SameSiteLaxMode

	ts := httptest.NewServer(api.NewServer(
		repo,
		api.WithAPIKeyAuthorizer(mustNewDBAuthorizer(t, repo)),
		api.WithSessionManager(sessionManager),
	).Handler())
	defer ts.Close()

	client := newSessionClient(t)
	loginResp := sessionPostJSON(t, client, ts.URL+"/v1/ui/sessions/login", map[string]any{
		"email":     "self-admin@tenant.test",
		"password":  "tenant self guard password 123",
		"tenant_id": "tenant_self_guard",
	}, "", http.StatusCreated)
	csrfToken, _ := loginResp["csrf_token"].(string)
	if csrfToken == "" {
		t.Fatalf("expected csrf token")
	}

	roleResp := sessionPatchJSON(t, client, ts.URL+"/v1/workspace/members/"+url.PathEscape(user.ID), map[string]any{
		"role": "writer",
	}, csrfToken, http.StatusForbidden)
	if got, _ := roleResp["error_code"].(string); got != "self_membership_mutation_forbidden" {
		t.Fatalf("expected self_membership_mutation_forbidden, got %#v", roleResp["error_code"])
	}

	deleteReq, err := http.NewRequest(http.MethodDelete, ts.URL+"/v1/workspace/members/"+url.PathEscape(user.ID), nil)
	if err != nil {
		t.Fatalf("build delete request: %v", err)
	}
	deleteReq.Header.Set("X-CSRF-Token", csrfToken)
	deleteResp, err := client.Do(deleteReq)
	if err != nil {
		t.Fatalf("delete request: %v", err)
	}
	defer deleteResp.Body.Close()
	if deleteResp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403 on self delete, got %d", deleteResp.StatusCode)
	}
	deletePayload := decodeResponseMap(t, deleteResp)
	if got, _ := deletePayload["error_code"].(string); got != "self_membership_mutation_forbidden" {
		t.Fatalf("expected self_membership_mutation_forbidden on delete, got %#v", deletePayload["error_code"])
	}
}

func decodeResponseMap(t *testing.T, resp *http.Response) map[string]any {
	t.Helper()
	defer resp.Body.Close()

	var payload map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response body: %v", err)
	}
	return payload
}

func mustNewDBAuthorizer(t *testing.T, repo *store.PostgresStore) *api.DBAPIKeyAuthorizer {
	t.Helper()
	authorizer, err := api.NewDBAPIKeyAuthorizer(repo)
	if err != nil {
		t.Fatalf("new authorizer: %v", err)
	}
	return authorizer
}
