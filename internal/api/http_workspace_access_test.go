package api_test

import (
	"database/sql"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

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

	authorizer, err := api.NewDBAPIKeyAuthorizer(repo)
	if err != nil {
		t.Fatalf("new authorizer: %v", err)
	}

	ts := httptest.NewServer(api.NewServer(repo, api.WithAPIKeyAuthorizer(authorizer)).Handler())
	defer ts.Close()

	members := getJSON(t, ts.URL+"/internal/tenants/tenant_access/members", "platform-admin", http.StatusOK)
	memberItems := members["items"].([]any)
	if len(memberItems) != 1 {
		t.Fatalf("expected 1 workspace member, got %d", len(memberItems))
	}
	member := memberItems[0].(map[string]any)
	if member["email"] != user.Email {
		t.Fatalf("expected member email %q, got %#v", user.Email, member["email"])
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
		"role": "writer",
	}, "platform-admin", http.StatusOK)
	memberPayload := updatedMember["member"].(map[string]any)
	if memberPayload["role"] != "writer" {
		t.Fatalf("expected updated member role writer, got %#v", memberPayload["role"])
	}

	revokeResult := postJSON(t, ts.URL+"/internal/tenants/tenant_access/invitations/"+inviteID+"/revoke", map[string]any{}, "platform-admin", http.StatusOK)
	revoked := revokeResult["invitation"].(map[string]any)
	if revoked["status"] != string(domain.WorkspaceInvitationStatusRevoked) {
		t.Fatalf("expected revoked invitation, got %#v", revoked["status"])
	}

	deleteRequest, err := http.NewRequest(http.MethodDelete, ts.URL+"/internal/tenants/tenant_access/members/"+user.ID, nil)
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
}
