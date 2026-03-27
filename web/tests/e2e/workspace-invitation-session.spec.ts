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
