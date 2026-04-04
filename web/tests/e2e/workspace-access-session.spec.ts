import { expect, test, type Page } from "@playwright/test";

type TenantSessionPayload = {
  authenticated: boolean;
  scope: "tenant";
  role: "admin" | "writer" | "reader";
  tenant_id: string;
  api_key_id: string;
  csrf_token: string;
};

const sessionPayload: TenantSessionPayload = {
  authenticated: true,
  scope: "tenant",
  role: "admin",
  tenant_id: "tenant_sagar",
  api_key_id: "api_key_admin_1",
  csrf_token: "csrf-tenant-123",
};

async function installWorkspaceAccessMock(page: Page) {
  await page.addInitScript(({ session }) => {
    const json = (status: number, payload: unknown) =>
      new Response(JSON.stringify(payload), {
        status,
        headers: {
          "Content-Type": "application/json",
        },
      });

    const serviceAccounts = [
      {
        id: "svc_bootstrap",
        workspace_id: "tenant_sagar",
        name: "bootstrap-admin-tenant-sagar",
        description: "Bootstrap admin machine identity",
        role: "admin",
        status: "active",
        purpose: "workspace bootstrap admin credential",
        environment: "prod",
        created_at: "2026-03-26T18:41:00Z",
        updated_at: "2026-03-26T18:41:00Z",
        active_credential_count: 1,
        credentials: [
          {
            id: "key_bootstrap_1",
            key_prefix: "dd5c18eb9829d5d4",
            name: "bootstrap-admin-tenant-sagar",
            role: "admin",
            tenant_id: "tenant_sagar",
            purpose: "workspace bootstrap admin credential",
            environment: "prod",
            created_at: "2026-03-26T18:41:00Z",
            last_used_at: undefined,
            revoked_at: undefined,
          },
        ],
      },
      {
        id: "svc_erp",
        workspace_id: "tenant_sagar",
        name: "erp-sync",
        description: "ERP export worker",
        role: "writer",
        status: "active",
        purpose: "erp sync",
        environment: "prod",
        created_at: "2026-03-27T08:00:00Z",
        updated_at: "2026-03-27T08:00:00Z",
        active_credential_count: 1,
        credentials: [
          {
            id: "key_erp_1",
            key_prefix: "ab12cd34ef56",
            name: "erp-sync-primary",
            role: "writer",
            tenant_id: "tenant_sagar",
            purpose: "erp sync",
            environment: "prod",
            created_at: "2026-03-27T08:00:00Z",
            last_used_at: "2026-03-27T10:30:00Z",
            revoked_at: undefined,
          },
          {
            id: "key_erp_old",
            key_prefix: "zz98yy76xx54",
            name: "erp-sync-legacy",
            role: "writer",
            tenant_id: "tenant_sagar",
            purpose: "erp sync",
            environment: "prod",
            created_at: "2026-03-20T08:00:00Z",
            last_used_at: "2026-03-24T10:30:00Z",
            revoked_at: "2026-03-25T08:00:00Z",
          },
        ],
      },
    ];

    const auditByServiceAccount: Record<string, unknown> = {
      svc_bootstrap: {
        service_account: serviceAccounts[0],
        items: [
          {
            id: "evt_created_bootstrap",
            tenant_id: "tenant_sagar",
            api_key_id: "key_bootstrap_1",
            action: "created",
            metadata: {
              owner_type: "service_account",
              owner_id: "svc_bootstrap",
              purpose: "workspace bootstrap admin credential",
              environment: "prod",
            },
            created_at: "2026-03-26T18:41:00Z",
          },
        ],
        total: 1,
        limit: 10,
        offset: 0,
      },
      svc_erp: {
        service_account: serviceAccounts[1],
        items: [
          {
            id: "evt_rotated_erp",
            tenant_id: "tenant_sagar",
            api_key_id: "key_erp_1",
            actor_api_key_id: "key_erp_old",
            action: "rotated",
            metadata: {
              new_api_key_id: "key_erp_1",
              owner_type: "service_account",
              owner_id: "svc_erp",
              purpose: "erp sync",
              environment: "prod",
            },
            created_at: "2026-03-27T09:00:00Z",
          },
          {
            id: "evt_revoked_erp",
            tenant_id: "tenant_sagar",
            api_key_id: "key_erp_old",
            action: "revoked",
            metadata: {
              owner_type: "service_account",
              owner_id: "svc_erp",
              purpose: "erp sync",
              environment: "prod",
            },
            created_at: "2026-03-25T08:00:00Z",
          },
        ],
        total: 2,
        limit: 10,
        offset: 0,
      },
    };

    const auditExportsByServiceAccount: Record<string, unknown> = {
      svc_bootstrap: {
        service_account: serviceAccounts[0],
        items: [],
        total: 0,
        limit: 5,
        offset: 0,
      },
      svc_erp: {
        service_account: serviceAccounts[1],
        items: [
          {
            job: {
              id: "job_export_1",
              tenant_id: "tenant_sagar",
              status: "completed",
              format: "csv",
              row_count: 14,
              created_at: "2026-03-27T09:10:00Z",
              updated_at: "2026-03-27T09:10:30Z",
            },
            download_url: "https://example.test/audit.csv",
          },
        ],
        total: 1,
        limit: 5,
        offset: 0,
      },
    };

    const originalFetch = window.fetch.bind(window);
    window.fetch = async (input, init) => {
      const request = input instanceof Request ? input : null;
      const method = (init?.method || request?.method || "GET").toUpperCase();
      const rawURL =
        typeof input === "string"
          ? input
          : input instanceof URL
            ? input.toString()
            : input.url;
      const url = new URL(rawURL, window.location.origin);
      const path = url.pathname;

      if (path === "/v1/ui/sessions/me" && method === "GET") {
        return json(200, session);
      }

      if (path === "/v1/workspace/members" && method === "GET") {
        return json(200, {
          items: [
            {
              user_id: "usr_admin_1",
              email: "admin@sagar.test",
              display_name: "Sagar Admin",
              role: "admin",
              status: "active",
              created_at: "2026-03-26T18:00:00Z",
              updated_at: "2026-03-26T18:00:00Z",
            },
          ],
        });
      }

      if (path === "/v1/workspace/invitations" && method === "GET") {
        return json(200, { items: [] });
      }

      if (path === "/v1/workspace/service-accounts" && method === "GET") {
        return json(200, { items: serviceAccounts });
      }

      if (path.startsWith("/v1/workspace/service-accounts/") && path.endsWith("/audit") && method === "GET") {
        const serviceAccountID = decodeURIComponent(path.split("/")[4] || "");
        return json(200, auditByServiceAccount[serviceAccountID] || { items: [], total: 0, limit: 10, offset: 0 });
      }

      if (path.startsWith("/v1/workspace/service-accounts/") && path.endsWith("/audit/exports") && method === "GET") {
        const serviceAccountID = decodeURIComponent(path.split("/")[4] || "");
        return json(200, auditExportsByServiceAccount[serviceAccountID] || { items: [], total: 0, limit: 5, offset: 0 });
      }

      if (path.startsWith("/v1/workspace/service-accounts/") && path.endsWith("/audit/exports") && method === "POST") {
        const serviceAccountID = decodeURIComponent(path.split("/")[4] || "");
        return json(201, {
          service_account: serviceAccounts.find((item) => item.id === serviceAccountID),
          idempotent_request: false,
          job: {
            id: "job_export_new",
            tenant_id: "tenant_sagar",
            status: "queued",
            format: "csv",
            row_count: 0,
            created_at: "2026-03-27T09:20:00Z",
            updated_at: "2026-03-27T09:20:00Z",
          },
        });
      }

      return originalFetch(input, init);
    };
  }, { session: sessionPayload });
}

test("workspace access shows summary-first service account and audit surfaces", async ({ page }) => {
  await installWorkspaceAccessMock(page);
  await page.goto("/workspace-access");

  // Members tab is active by default
  await expect(page.getByRole("tab", { name: "Members" })).toBeVisible();

  // Navigate to service accounts tab
  await page.getByRole("tab", { name: "Service accounts" }).click();
  await expect(page.getByText("API identities for automation and integrations. Issue or rotate credentials as needed.")).toBeVisible();
  await expect(page.getByText("created", { exact: true })).not.toBeVisible();

  await page.getByTestId("inspect-service-account-svc_erp").click();
  await expect(page.getByTestId("service-account-detail").getByText("ERP export worker")).toBeVisible();
  await expect(page.getByText(/last used/i).first()).toBeVisible();

  // Navigate to the Audit log tab directly
  await page.getByRole("tab", { name: "Audit log" }).click();
  await page.getByLabel("Audit service account").selectOption("svc_erp");
  await expect(page.getByText("Credential rotated")).toBeVisible();
  await page.getByRole("button", { name: /View service account audit details for Credential rotated/i }).click({ force: true });
  await expect(page.getByText("What happened")).toBeVisible();
});
