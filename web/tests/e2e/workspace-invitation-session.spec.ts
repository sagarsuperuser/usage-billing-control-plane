import { expect, test } from "@playwright/test";

test("authenticated invited user auto-accepts the workspace invitation", async ({ page, context }) => {
  let acceptCalls = 0;
  const tenantSession = {
    authenticated: true,
    scope: "tenant",
    role: "admin",
    tenant_id: "tenant_sagar",
    user_email: "sagar10018233@gmail.com",
    csrf_token: "csrf-invite-123",
  };

  await context.route("**/runtime-config", async (route) => {
    const url = new URL(route.request().url());
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({ apiBaseURL: url.origin }),
    });
  });

  await context.route("**/v1/ui/sessions/me", async (route) => {
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify(tenantSession),
    });
  });

  await context.route("**/v1/ui/invitations/invite-token-123", async (route) => {
    if (route.request().method() !== "GET") {
      await route.fallback();
      return;
    }
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({
        invitation: {
          id: "wsi_123",
          workspace_id: "tenant_sagar",
          email: "sagar10018233@gmail.com",
          role: "admin",
          status: "pending",
          expires_at: new Date(Date.now() + 3600_000).toISOString(),
          created_at: new Date().toISOString(),
          updated_at: new Date().toISOString(),
        },
        workspace_name: "Sagar Corp",
        requires_login: false,
        authenticated: true,
        current_user_email: "sagar10018233@gmail.com",
        email_matches_session: true,
        can_accept: true,
        account_exists: true,
      }),
    });
  });

  await context.route("**/v1/ui/invitations/invite-token-123/accept", async (route) => {
    acceptCalls += 1;
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({
        invitation: {
          id: "wsi_123",
          workspace_id: "tenant_sagar",
          email: "sagar10018233@gmail.com",
          role: "admin",
          status: "accepted",
          accepted_at: new Date().toISOString(),
          accepted_by_user_id: "usr_sagar",
          expires_at: new Date(Date.now() + 3600_000).toISOString(),
          created_at: new Date().toISOString(),
          updated_at: new Date().toISOString(),
        },
        session: tenantSession,
      }),
    });
  });

  await context.route("**/internal/customers**", async (route) => {
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify([]),
    });
  });

  await page.goto("/invite/invite-token-123");

  await expect.poll(() => acceptCalls).toBe(1);
  await expect(page).toHaveURL(/\/customers$/);
});

test("pending invitation login auto-accepts the workspace invitation", async ({ page, context }) => {
  let acceptCalls = 0;

  await context.route("**/runtime-config", async (route) => {
    const url = new URL(route.request().url());
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({ apiBaseURL: url.origin }),
    });
  });

  await context.route("**/v1/ui/sessions/me", async (route) => {
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({
        authenticated: true,
        subject_type: "user",
        subject_id: "usr_sagar",
        user_email: "sagar10018233@gmail.com",
        scope: "tenant",
        role: "admin",
        tenant_id: "tenant_sagar",
        csrf_token: "csrf-pending-123",
      }),
    });
  });



  await context.route("**/v1/ui/invitations/invite-token-pending", async (route) => {
    if (route.request().method() !== "GET") {
      await route.fallback();
      return;
    }
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({
        invitation: {
          id: "wsi_pending",
          workspace_id: "tenant_sagar",
          email: "sagar10018233@gmail.com",
          role: "admin",
          status: "pending",
          expires_at: new Date(Date.now() + 3600_000).toISOString(),
          created_at: new Date().toISOString(),
          updated_at: new Date().toISOString(),
        },
        workspace_name: "Sagar Corp",
        requires_login: false,
        authenticated: true,
        current_user_email: "sagar10018233@gmail.com",
        email_matches_session: true,
        can_accept: true,
        account_exists: true,
      }),
    });
  });

  await context.route("**/v1/ui/invitations/invite-token-pending/accept", async (route) => {
    acceptCalls += 1;
    expect(route.request().headers()["x-csrf-token"]).toBeTruthy();
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({
        invitation: {
          id: "wsi_pending",
          workspace_id: "tenant_sagar",
          email: "sagar10018233@gmail.com",
          role: "admin",
          status: "accepted",
          accepted_at: new Date().toISOString(),
          accepted_by_user_id: "usr_sagar",
          expires_at: new Date(Date.now() + 3600_000).toISOString(),
          created_at: new Date().toISOString(),
          updated_at: new Date().toISOString(),
        },
        session: {
          authenticated: true,
          scope: "tenant",
          role: "admin",
          tenant_id: "tenant_sagar",
          user_email: "sagar10018233@gmail.com",
          csrf_token: "csrf-final-123",
        },
      }),
    });
  });

  await context.route("**/internal/customers**", async (route) => {
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify([]),
    });
  });

  await page.goto("/invite/invite-token-pending");

  await expect.poll(() => acceptCalls).toBeGreaterThanOrEqual(1);
  await expect(page).toHaveURL(/\/control-plane|\/customers/);
});

test("unauthenticated invited user only sees explicit auth choices", async ({ page, context }) => {
  await context.route("**/runtime-config", async (route) => {
    const url = new URL(route.request().url());
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({ apiBaseURL: url.origin }),
    });
  });

  await context.route("**/v1/ui/sessions/me", async (route) => {
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: "null",
    });
  });

  await context.route("**/v1/ui/auth/providers", async (route) => {
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({
        sso_providers: [{ key: "google", display_name: "Google", type: "oidc" }],
      }),
    });
  });

  await context.route("**/v1/ui/invitations/invite-token-456", async (route) => {
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({
        invitation: {
          id: "wsi_456",
          workspace_id: "tenant_sagar",
          email: "sagar10018233@gmail.com",
          role: "admin",
          status: "pending",
          expires_at: new Date(Date.now() + 3600_000).toISOString(),
          created_at: new Date().toISOString(),
          updated_at: new Date().toISOString(),
        },
        workspace_name: "Sagar Corp",
        requires_login: true,
        authenticated: false,
        current_user_email: "",
        email_matches_session: false,
        can_accept: false,
        account_exists: true,
      }),
    });
  });

  await page.goto("/invite/invite-token-456");

  await expect(page.getByRole("link", { name: "Continue with Google" })).toBeVisible();
  await expect(page.getByRole("link", { name: "Sign in with email and password" })).toBeVisible();
  await expect(page.getByRole("link", { name: "Sign in to continue" })).toHaveCount(0);
});
