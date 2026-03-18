import { expect, test, type Page } from "@playwright/test";

type PlatformSessionPayload = {
  authenticated: boolean;
  scope: "platform";
  platform_role: "platform_admin";
  api_key_id: string;
  csrf_token: string;
};

type TenantRecord = {
  id: string;
  name: string;
  status: "active" | "suspended" | "deleted";
  billing_provider_connection_id?: string;
  workspace_billing: {
    configured: boolean;
    connected: boolean;
    active_billing_connection_id?: string;
    status: string;
    source?: string;
    isolation_mode?: "shared" | "dedicated";
  };
  created_at: string;
  updated_at: string;
};

type TenantOnboardingReadiness = {
  status: string;
  missing_steps: string[];
  tenant: {
    status: string;
    tenant_exists: boolean;
    tenant_active: boolean;
    tenant_admin_ready: boolean;
    missing_steps: string[];
  };
  billing_integration: {
    status: string;
    billing_mapping_ready: boolean;
    billing_connected?: boolean;
    workspace_billing_status?: string;
    workspace_billing_source?: string;
    active_billing_connection_id?: string;
    isolation_mode?: "shared" | "dedicated";
    pricing_ready: boolean;
    missing_steps: string[];
  };
  first_customer: {
    status: string;
    managed: boolean;
    customer_exists: boolean;
    customer_active: boolean;
    billing_profile_status: string;
    payment_setup_status: string;
    missing_steps: string[];
  };
};

type BillingProviderConnection = {
  id: string;
  display_name: string;
  status: string;
  lago_organization_id?: string;
  lago_provider_code?: string;
};

type WorkspaceMember = {
  user_id: string;
  email: string;
  display_name: string;
  role: "reader" | "writer" | "admin";
  status: "active" | "disabled";
  created_at: string;
  updated_at: string;
};

type WorkspaceInvitation = {
  id: string;
  workspace_id: string;
  email: string;
  role: "reader" | "writer" | "admin";
  status: "pending" | "accepted" | "expired" | "revoked";
  expires_at: string;
  invited_by_platform_user: boolean;
  created_at: string;
  updated_at: string;
  accept_url?: string;
};

function buildReadiness(pricingReady: boolean, customerExists: boolean, connectionID: string): TenantOnboardingReadiness {
  return {
    status: pricingReady && customerExists ? "ready" : "pending",
    missing_steps: [
      ...(pricingReady ? [] : ["billing_integration.pricing"]),
      ...(customerExists ? [] : ["first_customer.customer_created"]),
    ],
    tenant: {
      status: "ready",
      tenant_exists: true,
      tenant_active: true,
      tenant_admin_ready: true,
      missing_steps: [],
    },
    billing_integration: {
      status: pricingReady ? "ready" : "pending",
      billing_mapping_ready: true,
      billing_connected: true,
      workspace_billing_status: "connected",
      workspace_billing_source: "binding",
      active_billing_connection_id: connectionID,
      isolation_mode: "shared",
      pricing_ready: pricingReady,
      missing_steps: pricingReady ? [] : ["pricing"],
    },
    first_customer: {
      status: customerExists ? "ready" : "pending",
      managed: true,
      customer_exists: customerExists,
      customer_active: customerExists,
      billing_profile_status: customerExists ? "ready" : "missing",
      payment_setup_status: customerExists ? "ready" : "missing",
      missing_steps: customerExists ? [] : ["customer_created"],
    },
  };
}

async function installWorkspaceMock(page: Page, session: PlatformSessionPayload) {
  let loggedIn = false;
  const connections: Record<string, BillingProviderConnection> = {
    bpc_alpha: {
      id: "bpc_alpha",
      display_name: "Stripe Alpha",
      status: "connected",
    },
    bpc_beta: {
      id: "bpc_beta",
      display_name: "Stripe Beta",
      status: "connected",
    },
  };
  const tenants: TenantRecord[] = [
    {
      id: "tenant_alpha",
      name: "Tenant Alpha",
      status: "active",
      billing_provider_connection_id: "bpc_alpha",
      workspace_billing: {
        configured: true,
        connected: true,
        active_billing_connection_id: "bpc_alpha",
        status: "connected",
        source: "binding",
        isolation_mode: "shared",
      },
      created_at: new Date().toISOString(),
      updated_at: new Date().toISOString(),
    },
    {
      id: "tenant_beta",
      name: "Tenant Beta",
      status: "active",
      billing_provider_connection_id: "bpc_beta",
      workspace_billing: {
        configured: true,
        connected: true,
        active_billing_connection_id: "bpc_beta",
        status: "connected",
        source: "binding",
        isolation_mode: "shared",
      },
      created_at: new Date().toISOString(),
      updated_at: new Date().toISOString(),
    },
  ];

  const readinessByTenant: Record<string, TenantOnboardingReadiness> = {
    tenant_alpha: buildReadiness(false, false, "bpc_alpha"),
    tenant_beta: buildReadiness(true, true, "bpc_beta"),
  };
  const membersByTenant: Record<string, WorkspaceMember[]> = {
    tenant_alpha: [
      {
        user_id: "usr_owner_alpha",
        email: "owner@tenant-alpha.test",
        display_name: "Tenant Alpha Owner",
        role: "admin",
        status: "active",
        created_at: new Date().toISOString(),
        updated_at: new Date().toISOString(),
      },
    ],
    tenant_beta: [],
  };
  const invitationsByTenant: Record<string, WorkspaceInvitation[]> = {
    tenant_alpha: [],
    tenant_beta: [],
  };

  await page.route("**/*", async (route) => {
    const request = route.request();
    const url = new URL(request.url());
    const path = url.pathname;
    const method = request.method().toUpperCase();

    const json = async (status: number, payload: unknown) => {
      await route.fulfill({
        status,
        contentType: "application/json",
        body: JSON.stringify(payload),
      });
    };

    if (path === "/v1/ui/sessions/me" && method === "GET") {
      return json(loggedIn ? 200 : 401, loggedIn ? session : { error: "unauthorized" });
    }
    if (path === "/v1/ui/sessions/login" && method === "POST") {
      loggedIn = true;
      return json(201, session);
    }
    if (path === "/internal/tenants" && method === "GET") {
      return json(200, tenants);
    }
    if (path === "/internal/billing-provider-connections" && method === "GET") {
      return json(200, {
        items: Object.values(connections),
        total: Object.keys(connections).length,
      });
    }
    if (path.startsWith("/internal/onboarding/tenants/") && method === "GET") {
      const tenantID = decodeURIComponent(path.split("/").pop() || "");
      const tenant = tenants.find((item) => item.id === tenantID);
      if (!tenant) {
        return json(404, { error: "not found" });
      }
      return json(200, {
        tenant,
        readiness: readinessByTenant[tenantID],
        tenant_id: tenantID,
      });
    }
    if (path.startsWith("/internal/tenants/") && path.endsWith("/workspace-billing") && method === "PATCH") {
      const segments = path.split("/");
      const tenantID = decodeURIComponent(segments[3] || "");
      const tenant = tenants.find((item) => item.id === tenantID);
      if (!tenant) {
        return json(404, { error: "not found" });
      }
      const body = request.postDataJSON() as { billing_provider_connection_id?: string };
      const connectionID = body.billing_provider_connection_id || "";
      tenant.billing_provider_connection_id = connectionID;
      tenant.workspace_billing = {
        configured: Boolean(connectionID),
        connected: Boolean(connectionID),
        active_billing_connection_id: connectionID || undefined,
        status: connectionID ? "connected" : "missing",
        source: connectionID ? "binding" : "",
        isolation_mode: connectionID ? "shared" : undefined,
      };
      const readiness = readinessByTenant[tenantID];
      readiness.billing_integration.billing_connected = Boolean(connectionID);
      readiness.billing_integration.workspace_billing_status = connectionID ? "connected" : "missing";
      readiness.billing_integration.workspace_billing_source = connectionID ? "binding" : "";
      readiness.billing_integration.active_billing_connection_id = connectionID || undefined;
      readiness.billing_integration.isolation_mode = connectionID ? "shared" : undefined;
      return json(200, { tenant });
    }
    if (path.startsWith("/internal/tenants/") && path.endsWith("/members") && method === "GET") {
      const segments = path.split("/");
      const tenantID = decodeURIComponent(segments[3] || "");
      return json(200, { items: membersByTenant[tenantID] ?? [] });
    }
    if (path.startsWith("/internal/tenants/") && path.endsWith("/invitations") && method === "GET") {
      const segments = path.split("/");
      const tenantID = decodeURIComponent(segments[3] || "");
      return json(200, { items: invitationsByTenant[tenantID] ?? [] });
    }
    if (path.startsWith("/internal/tenants/") && path.endsWith("/invitations") && method === "POST") {
      const segments = path.split("/");
      const tenantID = decodeURIComponent(segments[3] || "");
      const body = request.postDataJSON() as { email?: string; role?: "reader" | "writer" | "admin" };
      const invitation: WorkspaceInvitation = {
        id: `wsi_${tenantID}_1`,
        workspace_id: tenantID,
        email: body.email || "",
        role: body.role || "admin",
        status: "pending",
        expires_at: new Date(Date.now() + 7 * 24 * 60 * 60 * 1000).toISOString(),
        invited_by_platform_user: true,
        created_at: new Date().toISOString(),
        updated_at: new Date().toISOString(),
        accept_url: `/invite/wsi_${tenantID}_token`,
      };
      invitationsByTenant[tenantID] = [invitation, ...(invitationsByTenant[tenantID] ?? [])];
      return json(201, {
        invitation,
        accept_url: invitation.accept_url,
        accept_path: `/invite/wsi_${tenantID}_token`,
      });
    }
    if (path.startsWith("/internal/tenants/") && path.includes("/invitations/") && path.endsWith("/revoke") && method === "POST") {
      const segments = path.split("/");
      const tenantID = decodeURIComponent(segments[3] || "");
      const invitationID = decodeURIComponent(segments[5] || "");
      const invitation = (invitationsByTenant[tenantID] ?? []).find((item) => item.id === invitationID);
      if (!invitation) {
        return json(404, { error: "not found" });
      }
      invitation.status = "revoked";
      invitation.updated_at = new Date().toISOString();
      return json(200, { invitation });
    }
    if (path.startsWith("/internal/billing-provider-connections/") && method === "GET") {
      const connectionID = decodeURIComponent(path.split("/").pop() || "");
      const connection = connections[connectionID];
      return json(connection ? 200 : 404, connection ? { connection } : { error: "not found" });
    }

    return route.continue();
  });
}

test("platform admin can browse workspaces and open workspace detail", async ({ page }) => {
  await installWorkspaceMock(page, {
    authenticated: true,
    scope: "platform",
    platform_role: "platform_admin",
    api_key_id: "platform_ui_1",
    csrf_token: "csrf-platform-123",
  });

  await page.goto("/workspaces");

  await page.getByTestId("session-login-email").fill("platform-admin@alpha.test");
  await page.getByTestId("session-login-password").fill("correct horse battery");
  await page.getByTestId("session-login-submit").click();

  await expect(page.getByRole("heading", { name: "Workspaces" })).toBeVisible();
  await expect(page.getByRole("link", { name: "New workspace" })).toBeVisible();
  await expect(page.getByRole("link", { name: /Tenant Alpha/i })).toBeVisible();
  await expect(page.getByText("Next action: pricing")).toBeVisible();

  await page.getByRole("link", { name: /Tenant Alpha/i }).click();
  await expect(page).toHaveURL(/\/workspaces\/tenant_alpha$/);
  await expect(page.getByRole("heading", { name: "Tenant Alpha" })).toBeVisible();
  await expect(page.getByText("Pricing rules still need to be configured").first()).toBeVisible();
  await expect(page.getByText("No billing-ready customer has been created yet").first()).toBeVisible();
  await expect(page.getByRole("link", { name: "Open billing connection" })).toBeVisible();
  await expect(page.getByText("Workspace access")).toBeVisible();
  await expect(page.getByText("Tenant Alpha Owner")).toBeVisible();
  await page.getByPlaceholder("tenant-admin@example.com").fill("new-admin@tenant-alpha.test");
  await page.getByLabel("Workspace role").selectOption("admin");
  await page.getByRole("button", { name: "Send invite" }).click();
  await expect(page.getByText("new-admin@tenant-alpha.test")).toBeVisible();
  await page.getByLabel("Active billing connection").selectOption("bpc_beta");
  await page.getByRole("button", { name: "Save active connection" }).click();
  await expect(page.getByText("bpc_beta")).toBeVisible();
});
