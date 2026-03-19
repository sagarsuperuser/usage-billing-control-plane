import {
  APIKey,
  APIKeyAuditEvent,
  APIKeyAuditExportJobResponse,
  BillingProviderConnection,
  BeginCustomerPaymentSetupResult,
  CustomerPaymentSetupRequestResult,
  CreateSubscriptionResult,
  Customer,
  Plan,
  PricingMetric,
  SubscriptionDetail,
  SubscriptionPaymentSetupResult,
  SubscriptionSummary,
  CustomerOnboardingResult,
  CustomerReadiness,
  CreditNoteSummary,
  InvoiceExplainability,
  InvoiceDetail,
  NotificationDispatchResult,
  PaymentDetail,
  PaymentFilters,
  PaymentReceiptSummary,
  PaymentSummary,
  InvoicePaymentLifecycle,
  InvoicePaymentStatusView,
  InvoicePaymentStatusSummary,
  InvoiceSummary,
  InvoiceStatusFilters,
  LagoWebhookEvent,
  ListResponse,
  ReplayJob,
  ReplayJobDiagnostics,
  RefreshCustomerPaymentSetupResult,
  RetryCustomerBillingProfileSyncResult,
  Tenant,
  TenantOnboardingReadiness,
  TenantOnboardingResult,
  UIAuthProviderList,
  UISession,
  WorkspaceInvitationIssueResult,
  WorkspaceInvitationPreview,
  WorkspaceInvitation,
  WorkspaceMember,
  WorkspaceSelectionState,
  ServiceAccount,
  ServiceAccountCredentialIssueResult,
} from "@/lib/types";

function trimTrailingSlash(value: string): string {
  return value.replace(/\/+$/, "");
}

export function getConfiguredAPIBaseURL(): string {
  if (typeof window !== "undefined") {
    const runtimeConfig = (window as Window & { __LAGO_ALPHA_RUNTIME__?: { apiBaseURL?: string } }).__LAGO_ALPHA_RUNTIME__;
    if (runtimeConfig?.apiBaseURL) {
      return trimTrailingSlash(runtimeConfig.apiBaseURL);
    }
  }
  return trimTrailingSlash(process.env.NEXT_PUBLIC_API_BASE_URL?.trim() ?? "");
}

export async function fetchRuntimeConfig(): Promise<{ apiBaseURL: string }> {
  const response = await fetch("/runtime-config", {
    method: "GET",
    cache: "no-store",
    credentials: "same-origin",
  });
  if (!response.ok) {
    throw new Error(`Failed to load runtime config (${response.status})`);
  }
  const payload = (await response.json()) as { apiBaseURL?: string };
  const apiBaseURL = trimTrailingSlash(payload.apiBaseURL?.trim() ?? "");
  if (typeof window !== "undefined") {
    (window as Window & { __LAGO_ALPHA_RUNTIME__?: { apiBaseURL?: string } }).__LAGO_ALPHA_RUNTIME__ = {
      apiBaseURL,
    };
  }
  return { apiBaseURL };
}

function resolveBaseURL(runtimeBaseURL?: string): string {
  const candidate = runtimeBaseURL?.trim() || getConfiguredAPIBaseURL();
  return trimTrailingSlash(candidate);
}

function toQuery(params: Record<string, string | number | boolean | undefined>) {
  const search = new URLSearchParams();
  for (const [key, value] of Object.entries(params)) {
    if (value === undefined || value === "") continue;
    search.set(key, String(value));
  }
  const raw = search.toString();
  return raw ? `?${raw}` : "";
}

async function apiRequest<T>(
  path: string,
  options: {
    method?: "GET" | "POST" | "PUT" | "PATCH" | "DELETE";
    runtimeBaseURL?: string;
    body?: unknown;
    csrfToken?: string;
    allowUnauthorized?: boolean;
  }
): Promise<T | null> {
  const baseURL = resolveBaseURL(options.runtimeBaseURL);
  const endpoint = baseURL ? `${baseURL}${path}` : path;

  const headers: Record<string, string> = {
    "Content-Type": "application/json",
  };
  if (options.csrfToken) {
    headers["X-CSRF-Token"] = options.csrfToken;
  }

  const response = await fetch(endpoint, {
    method: options.method ?? "GET",
    headers,
    body: options.body === undefined ? undefined : JSON.stringify(options.body),
    cache: "no-store",
    credentials: "include",
  });

  const isJSON = response.headers.get("content-type")?.includes("application/json");
  const payload = isJSON ? await response.json() : null;
  if (response.status === 401 && options.allowUnauthorized) {
    return null;
  }

  if (!response.ok) {
    const message =
      (payload && typeof payload.error === "string" && payload.error) ||
      `Request failed (${response.status})`;
    throw new Error(message);
  }

  return payload as T;
}

export class WorkspaceSelectionRequiredError extends Error {
  readonly selection: WorkspaceSelectionState;

  constructor(selection: WorkspaceSelectionState) {
    super("workspace selection required");
    this.name = "WorkspaceSelectionRequiredError";
    this.selection = selection;
  }
}

export function isWorkspaceSelectionRequiredError(value: unknown): value is WorkspaceSelectionRequiredError {
  return value instanceof WorkspaceSelectionRequiredError || (
    value instanceof Error &&
    value.name === "WorkspaceSelectionRequiredError"
  ) || (
    typeof value === "object" &&
    value !== null &&
    "selection" in value
  );
}

export class InvitationPendingLoginError extends Error {
  readonly nextPath: string;

  constructor(nextPath: string) {
    super("invitation login pending");
    this.name = "InvitationPendingLoginError";
    this.nextPath = nextPath;
  }
}

export function isInvitationPendingLoginError(value: unknown): value is InvitationPendingLoginError {
  return value instanceof InvitationPendingLoginError || (
    value instanceof Error &&
    value.name === "InvitationPendingLoginError"
  ) || (
    typeof value === "object" &&
    value !== null &&
    "nextPath" in value
  );
}

export async function loginUISession(input: {
  email: string;
  password: string;
  tenantID?: string;
  nextPath?: string;
  runtimeBaseURL?: string;
}): Promise<UISession> {
  const baseURL = resolveBaseURL(input.runtimeBaseURL);
  const endpoint = baseURL ? `${baseURL}/v1/ui/sessions/login` : "/v1/ui/sessions/login";
  const response = await fetch(endpoint, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
    },
    body: JSON.stringify({
      email: input.email,
      password: input.password,
      tenant_id: input.tenantID,
      next: input.nextPath,
    }),
    cache: "no-store",
    credentials: "include",
  });
  const isJSON = response.headers.get("content-type")?.includes("application/json");
  const payload = isJSON ? ((await response.json()) as Record<string, unknown>) : null;
  if (response.status === 409 && payload) {
    throw new WorkspaceSelectionRequiredError(payload as unknown as WorkspaceSelectionState);
  }
  if (response.status === 202 && payload && payload.pending_invitation === true) {
    throw new InvitationPendingLoginError(typeof payload.next_path === "string" ? payload.next_path : input.nextPath || "/");
  }
  if (!response.ok) {
    throw new Error((payload && typeof payload.error === "string" && payload.error) || `Request failed (${response.status})`);
  }
  return payload as unknown as UISession;
}

export async function fetchUISession(input: {
  runtimeBaseURL?: string;
}): Promise<UISession | null> {
  return apiRequest<UISession>("/v1/ui/sessions/me", {
    method: "GET",
    runtimeBaseURL: input.runtimeBaseURL,
    allowUnauthorized: true,
  });
}

export async function fetchUIAuthProviders(input: {
  runtimeBaseURL?: string;
}): Promise<UIAuthProviderList> {
  const payload = await apiRequest<UIAuthProviderList>("/v1/ui/auth/providers", {
    method: "GET",
    runtimeBaseURL: input.runtimeBaseURL,
  });
  if (!payload) {
    throw new Error("failed to load auth providers");
  }
  return payload;
}

export async function requestPasswordReset(input: {
  runtimeBaseURL?: string;
  email: string;
}): Promise<{ requested: boolean }> {
  const payload = await apiRequest<{ requested: boolean }>("/v1/ui/password/forgot", {
    method: "POST",
    runtimeBaseURL: input.runtimeBaseURL,
    body: {
      email: input.email,
    },
  });
  if (!payload) {
    throw new Error("password reset request failed");
  }
  return payload;
}

export async function resetPassword(input: {
  runtimeBaseURL?: string;
  token: string;
  password: string;
}): Promise<{ reset: boolean; user: { email: string; display_name: string } }> {
  const payload = await apiRequest<{ reset: boolean; user: { email: string; display_name: string } }>("/v1/ui/password/reset", {
    method: "POST",
    runtimeBaseURL: input.runtimeBaseURL,
    body: {
      token: input.token,
      password: input.password,
    },
  });
  if (!payload) {
    throw new Error("password reset failed");
  }
  return payload;
}

export async function logoutUISession(input: {
  runtimeBaseURL?: string;
  csrfToken: string;
}): Promise<void> {
  await apiRequest<{ logged_out: boolean }>("/v1/ui/sessions/logout", {
    method: "POST",
    runtimeBaseURL: input.runtimeBaseURL,
    csrfToken: input.csrfToken,
    body: {},
  });
}

export async function fetchPricingMetrics(input: {
  runtimeBaseURL?: string;
}): Promise<PricingMetric[]> {
  const payload = await apiRequest<PricingMetric[]>("/v1/pricing/metrics", {
    runtimeBaseURL: input.runtimeBaseURL,
    method: "GET",
  });
  if (!payload) {
    throw new Error("unauthorized");
  }
  return payload;
}

export async function createPricingMetric(input: {
  runtimeBaseURL?: string;
  csrfToken: string;
  body: Record<string, unknown>;
}): Promise<PricingMetric> {
  const payload = await apiRequest<PricingMetric>("/v1/pricing/metrics", {
    runtimeBaseURL: input.runtimeBaseURL,
    method: "POST",
    csrfToken: input.csrfToken,
    body: input.body,
  });
  if (!payload) {
    throw new Error("unauthorized");
  }
  return payload;
}

export async function fetchPricingMetric(input: {
  runtimeBaseURL?: string;
  metricID: string;
}): Promise<PricingMetric> {
  const payload = await apiRequest<PricingMetric>(`/v1/pricing/metrics/${encodeURIComponent(input.metricID)}`, {
    runtimeBaseURL: input.runtimeBaseURL,
    method: "GET",
  });
  if (!payload) {
    throw new Error("unauthorized");
  }
  return payload;
}

export async function fetchPlans(input: {
  runtimeBaseURL?: string;
}): Promise<Plan[]> {
  const payload = await apiRequest<Plan[]>("/v1/plans", {
    runtimeBaseURL: input.runtimeBaseURL,
    method: "GET",
  });
  if (!payload) {
    throw new Error("unauthorized");
  }
  return payload;
}

export async function createPlan(input: {
  runtimeBaseURL?: string;
  csrfToken: string;
  body: Record<string, unknown>;
}): Promise<Plan> {
  const payload = await apiRequest<Plan>("/v1/plans", {
    runtimeBaseURL: input.runtimeBaseURL,
    method: "POST",
    csrfToken: input.csrfToken,
    body: input.body,
  });
  if (!payload) {
    throw new Error("unauthorized");
  }
  return payload;
}

export async function fetchPlan(input: {
  runtimeBaseURL?: string;
  planID: string;
}): Promise<Plan> {
  const payload = await apiRequest<Plan>(`/v1/plans/${encodeURIComponent(input.planID)}`, {
    runtimeBaseURL: input.runtimeBaseURL,
    method: "GET",
  });
  if (!payload) {
    throw new Error("unauthorized");
  }
  return payload;
}

export async function fetchSubscriptions(input: {
  runtimeBaseURL?: string;
}): Promise<SubscriptionSummary[]> {
  const payload = await apiRequest<SubscriptionSummary[]>("/v1/subscriptions", {
    runtimeBaseURL: input.runtimeBaseURL,
    method: "GET",
  });
  if (!payload) {
    throw new Error("unauthorized");
  }
  return payload;
}

export async function createSubscription(input: {
  runtimeBaseURL?: string;
  csrfToken: string;
  body: Record<string, unknown>;
}): Promise<CreateSubscriptionResult> {
  const payload = await apiRequest<CreateSubscriptionResult>("/v1/subscriptions", {
    runtimeBaseURL: input.runtimeBaseURL,
    method: "POST",
    csrfToken: input.csrfToken,
    body: input.body,
  });
  if (!payload) {
    throw new Error("unauthorized");
  }
  return payload;
}

export async function fetchSubscription(input: {
  runtimeBaseURL?: string;
  subscriptionID: string;
}): Promise<SubscriptionDetail> {
  const payload = await apiRequest<SubscriptionDetail>(`/v1/subscriptions/${encodeURIComponent(input.subscriptionID)}`, {
    runtimeBaseURL: input.runtimeBaseURL,
    method: "GET",
  });
  if (!payload) {
    throw new Error("unauthorized");
  }
  return payload;
}

export async function requestSubscriptionPaymentSetup(input: {
  runtimeBaseURL?: string;
  csrfToken: string;
  subscriptionID: string;
  paymentMethodType?: string;
}): Promise<SubscriptionPaymentSetupResult> {
  const payload = await apiRequest<SubscriptionPaymentSetupResult>(
    `/v1/subscriptions/${encodeURIComponent(input.subscriptionID)}/payment-setup/request`,
    {
      runtimeBaseURL: input.runtimeBaseURL,
      method: "POST",
      csrfToken: input.csrfToken,
      body: { payment_method_type: input.paymentMethodType },
    }
  );
  if (!payload) {
    throw new Error("unauthorized");
  }
  return payload;
}

export async function resendSubscriptionPaymentSetup(input: {
  runtimeBaseURL?: string;
  csrfToken: string;
  subscriptionID: string;
  paymentMethodType?: string;
}): Promise<SubscriptionPaymentSetupResult> {
  const payload = await apiRequest<SubscriptionPaymentSetupResult>(
    `/v1/subscriptions/${encodeURIComponent(input.subscriptionID)}/payment-setup/resend`,
    {
      runtimeBaseURL: input.runtimeBaseURL,
      method: "POST",
      csrfToken: input.csrfToken,
      body: { payment_method_type: input.paymentMethodType },
    }
  );
  if (!payload) {
    throw new Error("unauthorized");
  }
  return payload;
}

export async function fetchTenants(input: {
  runtimeBaseURL?: string;
  status?: string;
}): Promise<Tenant[]> {
  const query = toQuery({
    status: input.status,
  });
  const payload = await apiRequest<Tenant[]>(`/internal/tenants${query}`, {
    runtimeBaseURL: input.runtimeBaseURL,
    method: "GET",
  });
  if (!payload) {
    throw new Error("unauthorized");
  }
  return payload;
}

export async function onboardTenant(input: {
  runtimeBaseURL?: string;
  csrfToken: string;
  body: Record<string, unknown>;
}): Promise<TenantOnboardingResult> {
  const payload = await apiRequest<TenantOnboardingResult>("/internal/onboarding/tenants", {
    runtimeBaseURL: input.runtimeBaseURL,
    method: "POST",
    csrfToken: input.csrfToken,
    body: input.body,
  });
  if (!payload) {
    throw new Error("unauthorized");
  }
  return payload;
}

export async function fetchTenantOnboardingStatus(input: {
  runtimeBaseURL?: string;
  tenantID: string;
}): Promise<{ tenant: Tenant; readiness: TenantOnboardingReadiness; tenant_id: string }> {
  const payload = await apiRequest<{ tenant: Tenant; readiness: TenantOnboardingReadiness; tenant_id: string }>(
    `/internal/onboarding/tenants/${encodeURIComponent(input.tenantID)}`,
    {
      runtimeBaseURL: input.runtimeBaseURL,
      method: "GET",
    }
  );
  if (!payload) {
    throw new Error("unauthorized");
  }
  return payload;
}

export async function fetchBillingProviderConnections(input: {
  runtimeBaseURL?: string;
  providerType?: string;
  environment?: string;
  status?: string;
  scope?: string;
  ownerTenantID?: string;
  limit?: number;
  offset?: number;
}): Promise<BillingProviderConnection[]> {
  const query = toQuery({
    provider_type: input.providerType,
    environment: input.environment,
    status: input.status,
    scope: input.scope,
    owner_tenant_id: input.ownerTenantID,
    limit: input.limit,
    offset: input.offset,
  });
  const payload = await apiRequest<ListResponse<BillingProviderConnection>>(
    `/internal/billing-provider-connections${query}`,
    {
      runtimeBaseURL: input.runtimeBaseURL,
      method: "GET",
    }
  );
  if (!payload) {
    throw new Error("unauthorized");
  }
  return payload.items;
}

export async function fetchBillingProviderConnection(input: {
  runtimeBaseURL?: string;
  connectionID: string;
}): Promise<BillingProviderConnection> {
  const payload = await apiRequest<{ connection: BillingProviderConnection }>(
    `/internal/billing-provider-connections/${encodeURIComponent(input.connectionID)}`,
    {
      runtimeBaseURL: input.runtimeBaseURL,
      method: "GET",
    }
  );
  if (!payload) {
    throw new Error("unauthorized");
  }
  return payload.connection;
}

export async function createBillingProviderConnection(input: {
  runtimeBaseURL?: string;
  csrfToken: string;
  body: Record<string, unknown>;
}): Promise<BillingProviderConnection> {
  const payload = await apiRequest<{ connection: BillingProviderConnection }>(
    "/internal/billing-provider-connections",
    {
      runtimeBaseURL: input.runtimeBaseURL,
      method: "POST",
      csrfToken: input.csrfToken,
      body: input.body,
    }
  );
  if (!payload) {
    throw new Error("unauthorized");
  }
  return payload.connection;
}

export async function updateBillingProviderConnection(input: {
  runtimeBaseURL?: string;
  csrfToken: string;
  connectionID: string;
  body: Record<string, unknown>;
}): Promise<BillingProviderConnection> {
  const payload = await apiRequest<{ connection: BillingProviderConnection }>(
    `/internal/billing-provider-connections/${encodeURIComponent(input.connectionID)}`,
    {
      runtimeBaseURL: input.runtimeBaseURL,
      method: "PATCH",
      csrfToken: input.csrfToken,
      body: input.body,
    }
  );
  if (!payload) {
    throw new Error("unauthorized");
  }
  return payload.connection;
}

export async function updateTenantWorkspaceBilling(input: {
  runtimeBaseURL?: string;
  csrfToken: string;
  tenantID: string;
  billingProviderConnectionID: string;
}): Promise<Tenant> {
  const payload = await apiRequest<{ tenant: Tenant }>(
    `/internal/tenants/${encodeURIComponent(input.tenantID)}/workspace-billing`,
    {
      runtimeBaseURL: input.runtimeBaseURL,
      method: "PATCH",
      csrfToken: input.csrfToken,
      body: {
        billing_provider_connection_id: input.billingProviderConnectionID,
      },
    }
  );
  if (!payload) {
    throw new Error("unauthorized");
  }
  return payload.tenant;
}

export async function fetchWorkspaceMembers(input: {
  runtimeBaseURL?: string;
  tenantID: string;
}): Promise<WorkspaceMember[]> {
  const payload = await apiRequest<{ items: WorkspaceMember[] }>(
    `/internal/tenants/${encodeURIComponent(input.tenantID)}/members`,
    {
      runtimeBaseURL: input.runtimeBaseURL,
      method: "GET",
    }
  );
  if (!payload) {
    throw new Error("unauthorized");
  }
  return payload.items;
}

export async function updateWorkspaceMember(input: {
  runtimeBaseURL?: string;
  csrfToken: string;
  tenantID: string;
  userID: string;
  role: "reader" | "writer" | "admin";
}): Promise<WorkspaceMember> {
  const payload = await apiRequest<{ member: WorkspaceMember }>(
    `/internal/tenants/${encodeURIComponent(input.tenantID)}/members/${encodeURIComponent(input.userID)}`,
    {
      runtimeBaseURL: input.runtimeBaseURL,
      method: "PATCH",
      csrfToken: input.csrfToken,
      body: {
        role: input.role,
      },
    }
  );
  if (!payload) {
    throw new Error("unauthorized");
  }
  return payload.member;
}

export async function removeWorkspaceMember(input: {
  runtimeBaseURL?: string;
  csrfToken: string;
  tenantID: string;
  userID: string;
}): Promise<void> {
  await apiRequest<null>(
    `/internal/tenants/${encodeURIComponent(input.tenantID)}/members/${encodeURIComponent(input.userID)}`,
    {
      runtimeBaseURL: input.runtimeBaseURL,
      method: "DELETE",
      csrfToken: input.csrfToken,
    }
  );
}

export async function fetchWorkspaceInvitations(input: {
  runtimeBaseURL?: string;
  tenantID: string;
  status?: string;
}): Promise<WorkspaceInvitation[]> {
  const query = toQuery({ status: input.status });
  const payload = await apiRequest<{ items: WorkspaceInvitation[] }>(
    `/internal/tenants/${encodeURIComponent(input.tenantID)}/invitations${query}`,
    {
      runtimeBaseURL: input.runtimeBaseURL,
      method: "GET",
    }
  );
  if (!payload) {
    throw new Error("unauthorized");
  }
  return payload.items;
}

export async function createWorkspaceInvitation(input: {
  runtimeBaseURL?: string;
  csrfToken: string;
  tenantID: string;
  email: string;
  role: "reader" | "writer" | "admin";
}): Promise<WorkspaceInvitationIssueResult> {
  const payload = await apiRequest<WorkspaceInvitationIssueResult>(
    `/internal/tenants/${encodeURIComponent(input.tenantID)}/invitations`,
    {
      runtimeBaseURL: input.runtimeBaseURL,
      method: "POST",
      csrfToken: input.csrfToken,
      body: {
        email: input.email,
        role: input.role,
      },
    }
  );
  if (!payload) {
    throw new Error("unauthorized");
  }
  return payload;
}

export async function revokeWorkspaceInvitation(input: {
  runtimeBaseURL?: string;
  csrfToken: string;
  tenantID: string;
  invitationID: string;
}): Promise<WorkspaceInvitation> {
  const payload = await apiRequest<{ invitation: WorkspaceInvitation }>(
    `/internal/tenants/${encodeURIComponent(input.tenantID)}/invitations/${encodeURIComponent(input.invitationID)}/revoke`,
    {
      runtimeBaseURL: input.runtimeBaseURL,
      method: "POST",
      csrfToken: input.csrfToken,
      body: {},
    }
  );
  if (!payload) {
    throw new Error("unauthorized");
  }
  return payload.invitation;
}

export async function fetchTenantWorkspaceMembers(input: {
  runtimeBaseURL?: string;
}): Promise<WorkspaceMember[]> {
  const payload = await apiRequest<{ items: WorkspaceMember[] }>("/v1/workspace/members", {
    runtimeBaseURL: input.runtimeBaseURL,
    method: "GET",
  });
  if (!payload) {
    throw new Error("unauthorized");
  }
  return payload.items;
}

export async function updateTenantWorkspaceMember(input: {
  runtimeBaseURL?: string;
  csrfToken: string;
  userID: string;
  role: "reader" | "writer" | "admin";
}): Promise<WorkspaceMember> {
  const payload = await apiRequest<{ member: WorkspaceMember }>(`/v1/workspace/members/${encodeURIComponent(input.userID)}`, {
    runtimeBaseURL: input.runtimeBaseURL,
    method: "PATCH",
    csrfToken: input.csrfToken,
    body: {
      role: input.role,
    },
  });
  if (!payload) {
    throw new Error("unauthorized");
  }
  return payload.member;
}

export async function removeTenantWorkspaceMember(input: {
  runtimeBaseURL?: string;
  csrfToken: string;
  userID: string;
}): Promise<void> {
  await apiRequest<null>(`/v1/workspace/members/${encodeURIComponent(input.userID)}`, {
    runtimeBaseURL: input.runtimeBaseURL,
    method: "DELETE",
    csrfToken: input.csrfToken,
  });
}

export async function fetchTenantWorkspaceInvitations(input: {
  runtimeBaseURL?: string;
  status?: string;
}): Promise<WorkspaceInvitation[]> {
  const query = toQuery({ status: input.status });
  const payload = await apiRequest<{ items: WorkspaceInvitation[] }>(`/v1/workspace/invitations${query}`, {
    runtimeBaseURL: input.runtimeBaseURL,
    method: "GET",
  });
  if (!payload) {
    throw new Error("unauthorized");
  }
  return payload.items;
}

export async function createTenantWorkspaceInvitation(input: {
  runtimeBaseURL?: string;
  csrfToken: string;
  email: string;
  role: "reader" | "writer" | "admin";
}): Promise<WorkspaceInvitationIssueResult> {
  const payload = await apiRequest<WorkspaceInvitationIssueResult>("/v1/workspace/invitations", {
    runtimeBaseURL: input.runtimeBaseURL,
    method: "POST",
    csrfToken: input.csrfToken,
    body: {
      email: input.email,
      role: input.role,
    },
  });
  if (!payload) {
    throw new Error("unauthorized");
  }
  return payload;
}

export async function revokeTenantWorkspaceInvitation(input: {
  runtimeBaseURL?: string;
  csrfToken: string;
  invitationID: string;
}): Promise<WorkspaceInvitation> {
  const payload = await apiRequest<{ invitation: WorkspaceInvitation }>(
    `/v1/workspace/invitations/${encodeURIComponent(input.invitationID)}/revoke`,
    {
      runtimeBaseURL: input.runtimeBaseURL,
      method: "POST",
      csrfToken: input.csrfToken,
      body: {},
    }
  );
  if (!payload) {
    throw new Error("unauthorized");
  }
  return payload.invitation;
}

export async function fetchTenantWorkspaceServiceAccounts(input: {
  runtimeBaseURL?: string;
}): Promise<ServiceAccount[]> {
  const payload = await apiRequest<{ items: ServiceAccount[] }>("/v1/workspace/service-accounts", {
    runtimeBaseURL: input.runtimeBaseURL,
    method: "GET",
  });
  if (!payload) {
    throw new Error("unauthorized");
  }
  return payload.items;
}

export async function createTenantWorkspaceServiceAccount(input: {
  runtimeBaseURL?: string;
  csrfToken: string;
  name: string;
  description?: string;
  role: "reader" | "writer" | "admin";
  purpose?: string;
  environment?: string;
  issueInitialCredential?: boolean;
  credentialName?: string;
}): Promise<{ service_account: ServiceAccount; credential?: APIKey; secret?: string }> {
  const payload = await apiRequest<{ service_account: ServiceAccount; credential?: APIKey; secret?: string }>("/v1/workspace/service-accounts", {
    runtimeBaseURL: input.runtimeBaseURL,
    method: "POST",
    csrfToken: input.csrfToken,
    body: {
      name: input.name,
      description: input.description,
      role: input.role,
      purpose: input.purpose,
      environment: input.environment,
      issue_initial_credential: input.issueInitialCredential,
      credential_name: input.credentialName,
    },
  });
  if (!payload) {
    throw new Error("unauthorized");
  }
  return payload;
}

export async function issueTenantWorkspaceServiceAccountCredential(input: {
  runtimeBaseURL?: string;
  csrfToken: string;
  serviceAccountID: string;
  name?: string;
}): Promise<ServiceAccountCredentialIssueResult> {
  const payload = await apiRequest<ServiceAccountCredentialIssueResult>(`/v1/workspace/service-accounts/${encodeURIComponent(input.serviceAccountID)}/credentials`, {
    runtimeBaseURL: input.runtimeBaseURL,
    method: "POST",
    csrfToken: input.csrfToken,
    body: {
      name: input.name,
    },
  });
  if (!payload) {
    throw new Error("unauthorized");
  }
  return payload;
}

export async function rotateTenantWorkspaceServiceAccountCredential(input: {
  runtimeBaseURL?: string;
  csrfToken: string;
  serviceAccountID: string;
  credentialID: string;
}): Promise<ServiceAccountCredentialIssueResult> {
  const payload = await apiRequest<ServiceAccountCredentialIssueResult>(`/v1/workspace/service-accounts/${encodeURIComponent(input.serviceAccountID)}/credentials/${encodeURIComponent(input.credentialID)}/rotate`, {
    runtimeBaseURL: input.runtimeBaseURL,
    method: "POST",
    csrfToken: input.csrfToken,
    body: {},
  });
  if (!payload) {
    throw new Error("unauthorized");
  }
  return payload;
}

export async function revokeTenantWorkspaceServiceAccountCredential(input: {
  runtimeBaseURL?: string;
  csrfToken: string;
  serviceAccountID: string;
  credentialID: string;
}): Promise<APIKey> {
  const payload = await apiRequest<{ credential: APIKey }>(`/v1/workspace/service-accounts/${encodeURIComponent(input.serviceAccountID)}/credentials/${encodeURIComponent(input.credentialID)}/revoke`, {
    runtimeBaseURL: input.runtimeBaseURL,
    method: "POST",
    csrfToken: input.csrfToken,
    body: {},
  });
  if (!payload) {
    throw new Error("unauthorized");
  }
  return payload.credential;
}

export async function updateTenantWorkspaceServiceAccountStatus(input: {
  runtimeBaseURL?: string;
  csrfToken: string;
  serviceAccountID: string;
  status: "active" | "disabled";
}): Promise<ServiceAccount> {
  const payload = await apiRequest<{ service_account: ServiceAccount }>(`/v1/workspace/service-accounts/${encodeURIComponent(input.serviceAccountID)}`, {
    runtimeBaseURL: input.runtimeBaseURL,
    method: "PATCH",
    csrfToken: input.csrfToken,
    body: {
      status: input.status,
    },
  });
  if (!payload) {
    throw new Error("unauthorized");
  }
  return payload.service_account;
}

export async function fetchTenantWorkspaceServiceAccountAudit(input: {
  runtimeBaseURL?: string;
  serviceAccountID: string;
  limit?: number;
  cursor?: string;
}): Promise<{ service_account: ServiceAccount; items: APIKeyAuditEvent[]; total: number; limit: number; offset: number; next_cursor?: string }> {
  const query = toQuery({
    limit: input.limit,
    cursor: input.cursor,
  });
  const payload = await apiRequest<{ service_account: ServiceAccount; items: APIKeyAuditEvent[]; total: number; limit: number; offset: number; next_cursor?: string }>(
    `/v1/workspace/service-accounts/${encodeURIComponent(input.serviceAccountID)}/audit${query ? `?${query}` : ""}`,
    {
      runtimeBaseURL: input.runtimeBaseURL,
      method: "GET",
    }
  );
  if (!payload) {
    throw new Error("unauthorized");
  }
  return payload;
}

export async function fetchTenantWorkspaceServiceAccountAuditExports(input: {
  runtimeBaseURL?: string;
  serviceAccountID: string;
  limit?: number;
  cursor?: string;
}): Promise<{ service_account: ServiceAccount; items: APIKeyAuditExportJobResponse[]; total: number; limit: number; offset: number; next_cursor?: string }> {
  const query = toQuery({
    limit: input.limit,
    cursor: input.cursor,
  });
  const payload = await apiRequest<{ service_account: ServiceAccount; items: APIKeyAuditExportJobResponse[]; total: number; limit: number; offset: number; next_cursor?: string }>(
    `/v1/workspace/service-accounts/${encodeURIComponent(input.serviceAccountID)}/audit/exports${query ? `?${query}` : ""}`,
    {
      runtimeBaseURL: input.runtimeBaseURL,
      method: "GET",
    }
  );
  if (!payload) {
    throw new Error("unauthorized");
  }
  return payload;
}

export async function createTenantWorkspaceServiceAccountAuditExport(input: {
  runtimeBaseURL?: string;
  csrfToken: string;
  serviceAccountID: string;
  idempotencyKey: string;
  action?: string;
}): Promise<{ service_account: ServiceAccount; idempotent_request: boolean; job: APIKeyAuditExportJobResponse["job"] }> {
  const payload = await apiRequest<{ service_account: ServiceAccount; idempotent_request: boolean; job: APIKeyAuditExportJobResponse["job"] }>(
    `/v1/workspace/service-accounts/${encodeURIComponent(input.serviceAccountID)}/audit/exports`,
    {
      runtimeBaseURL: input.runtimeBaseURL,
      method: "POST",
      csrfToken: input.csrfToken,
      body: {
        idempotency_key: input.idempotencyKey,
        action: input.action,
      },
    }
  );
  if (!payload) {
    throw new Error("unauthorized");
  }
  return payload;
}

export async function fetchPendingWorkspaceSelection(input: {
  runtimeBaseURL?: string;
}): Promise<WorkspaceSelectionState> {
  const payload = await apiRequest<WorkspaceSelectionState>("/v1/ui/workspaces/pending", {
    runtimeBaseURL: input.runtimeBaseURL,
    method: "GET",
  });
  if (!payload) {
    throw new Error("workspace selection not pending");
  }
  return payload;
}

export async function selectPendingWorkspace(input: {
  runtimeBaseURL?: string;
  csrfToken: string;
  tenantID: string;
}): Promise<UISession> {
  const payload = await apiRequest<UISession>("/v1/ui/workspaces/select", {
    runtimeBaseURL: input.runtimeBaseURL,
    method: "POST",
    csrfToken: input.csrfToken,
    body: {
      tenant_id: input.tenantID,
    },
  });
  if (!payload) {
    throw new Error("workspace selection failed");
  }
  return payload;
}

export async function fetchWorkspaceInvitationPreview(input: {
  runtimeBaseURL?: string;
  token: string;
}): Promise<WorkspaceInvitationPreview> {
  const payload = await apiRequest<WorkspaceInvitationPreview>(`/v1/ui/invitations/${encodeURIComponent(input.token)}`, {
    runtimeBaseURL: input.runtimeBaseURL,
    method: "GET",
  });
  if (!payload) {
    throw new Error("workspace invitation not found");
  }
  return payload;
}

export async function acceptWorkspaceInvitation(input: {
  runtimeBaseURL?: string;
  csrfToken: string;
  token: string;
}): Promise<{ invitation: WorkspaceInvitation; session: UISession }> {
  const payload = await apiRequest<{ invitation: WorkspaceInvitation; session: UISession }>(
    `/v1/ui/invitations/${encodeURIComponent(input.token)}/accept`,
    {
      runtimeBaseURL: input.runtimeBaseURL,
      method: "POST",
      csrfToken: input.csrfToken,
      body: {},
    }
  );
  if (!payload) {
    throw new Error("workspace invitation acceptance failed");
  }
  return payload;
}

export async function registerWorkspaceInvitation(input: {
  runtimeBaseURL?: string;
  token: string;
  displayName?: string;
  password: string;
}): Promise<{ invitation: WorkspaceInvitation; session: UISession }> {
  const payload = await apiRequest<{ invitation: WorkspaceInvitation; session: UISession }>(
    `/v1/ui/invitations/${encodeURIComponent(input.token)}/register`,
    {
      runtimeBaseURL: input.runtimeBaseURL,
      method: "POST",
      body: {
        display_name: input.displayName?.trim() || "",
        password: input.password,
      },
    }
  );
  if (!payload) {
    throw new Error("workspace invitation registration failed");
  }
  return payload;
}

export async function syncBillingProviderConnection(input: {
  runtimeBaseURL?: string;
  csrfToken: string;
  connectionID: string;
}): Promise<BillingProviderConnection> {
  const payload = await apiRequest<{ connection: BillingProviderConnection }>(
    `/internal/billing-provider-connections/${encodeURIComponent(input.connectionID)}/sync`,
    {
      runtimeBaseURL: input.runtimeBaseURL,
      method: "POST",
      csrfToken: input.csrfToken,
      body: {},
    }
  );
  if (!payload) {
    throw new Error("unauthorized");
  }
  return payload.connection;
}

export async function disableBillingProviderConnection(input: {
  runtimeBaseURL?: string;
  csrfToken: string;
  connectionID: string;
}): Promise<BillingProviderConnection> {
  const payload = await apiRequest<{ connection: BillingProviderConnection }>(
    `/internal/billing-provider-connections/${encodeURIComponent(input.connectionID)}/disable`,
    {
      runtimeBaseURL: input.runtimeBaseURL,
      method: "POST",
      csrfToken: input.csrfToken,
      body: {},
    }
  );
  if (!payload) {
    throw new Error("unauthorized");
  }
  return payload.connection;
}

export async function onboardCustomer(input: {
  runtimeBaseURL?: string;
  csrfToken: string;
  body: Record<string, unknown>;
}): Promise<CustomerOnboardingResult> {
  const payload = await apiRequest<CustomerOnboardingResult>("/v1/customer-onboarding", {
    runtimeBaseURL: input.runtimeBaseURL,
    method: "POST",
    csrfToken: input.csrfToken,
    body: input.body,
  });
  if (!payload) {
    throw new Error("unauthorized");
  }
  return payload;
}

export async function fetchCustomers(input: {
  runtimeBaseURL?: string;
  status?: string;
  externalID?: string;
  limit?: number;
  offset?: number;
}): Promise<Customer[]> {
  const query = toQuery({
    status: input.status,
    external_id: input.externalID,
    limit: input.limit,
    offset: input.offset,
  });
  const payload = await apiRequest<Customer[]>(`/v1/customers${query}`, {
    runtimeBaseURL: input.runtimeBaseURL,
    method: "GET",
  });
  if (!payload) {
    throw new Error("unauthorized");
  }
  return payload;
}

export async function fetchCustomerReadiness(input: {
  runtimeBaseURL?: string;
  externalID: string;
}): Promise<CustomerReadiness> {
  const payload = await apiRequest<CustomerReadiness>(
    `/v1/customers/${encodeURIComponent(input.externalID)}/readiness`,
    {
      runtimeBaseURL: input.runtimeBaseURL,
      method: "GET",
    }
  );
  if (!payload) {
    throw new Error("unauthorized");
  }
  return payload;
}

export async function retryCustomerBillingSync(input: {
  runtimeBaseURL?: string;
  csrfToken: string;
  externalID: string;
}): Promise<RetryCustomerBillingProfileSyncResult> {
  const payload = await apiRequest<RetryCustomerBillingProfileSyncResult>(
    `/v1/customers/${encodeURIComponent(input.externalID)}/billing-profile/retry-sync`,
    {
      runtimeBaseURL: input.runtimeBaseURL,
      method: "POST",
      csrfToken: input.csrfToken,
      body: {},
    }
  );
  if (!payload) {
    throw new Error("unauthorized");
  }
  return payload;
}

export async function refreshCustomerPaymentSetup(input: {
  runtimeBaseURL?: string;
  csrfToken: string;
  externalID: string;
}): Promise<RefreshCustomerPaymentSetupResult> {
  const payload = await apiRequest<RefreshCustomerPaymentSetupResult>(
    `/v1/customers/${encodeURIComponent(input.externalID)}/payment-setup/refresh`,
    {
      runtimeBaseURL: input.runtimeBaseURL,
      method: "POST",
      csrfToken: input.csrfToken,
      body: {},
    }
  );
  if (!payload) {
    throw new Error("unauthorized");
  }
  return payload;
}

export async function beginCustomerPaymentSetup(input: {
  runtimeBaseURL?: string;
  csrfToken: string;
  externalID: string;
  paymentMethodType?: string;
}): Promise<BeginCustomerPaymentSetupResult> {
  const payload = await apiRequest<BeginCustomerPaymentSetupResult>(
    `/v1/customers/${encodeURIComponent(input.externalID)}/payment-setup/checkout-url`,
    {
      runtimeBaseURL: input.runtimeBaseURL,
      method: "POST",
      csrfToken: input.csrfToken,
      body: {
        payment_method_type: input.paymentMethodType,
      },
    }
  );
  if (!payload) {
    throw new Error("unauthorized");
  }
  return payload;
}

export async function requestCustomerPaymentSetup(input: {
  runtimeBaseURL?: string;
  csrfToken: string;
  externalID: string;
  paymentMethodType?: string;
}): Promise<CustomerPaymentSetupRequestResult> {
  const payload = await apiRequest<CustomerPaymentSetupRequestResult>(
    `/v1/customers/${encodeURIComponent(input.externalID)}/payment-setup/request`,
    {
      runtimeBaseURL: input.runtimeBaseURL,
      method: "POST",
      csrfToken: input.csrfToken,
      body: {
        payment_method_type: input.paymentMethodType,
      },
    }
  );
  if (!payload) {
    throw new Error("unauthorized");
  }
  return payload;
}

export async function resendCustomerPaymentSetup(input: {
  runtimeBaseURL?: string;
  csrfToken: string;
  externalID: string;
  paymentMethodType?: string;
}): Promise<CustomerPaymentSetupRequestResult> {
  const payload = await apiRequest<CustomerPaymentSetupRequestResult>(
    `/v1/customers/${encodeURIComponent(input.externalID)}/payment-setup/resend`,
    {
      runtimeBaseURL: input.runtimeBaseURL,
      method: "POST",
      csrfToken: input.csrfToken,
      body: {
        payment_method_type: input.paymentMethodType,
      },
    }
  );
  if (!payload) {
    throw new Error("unauthorized");
  }
  return payload;
}


export async function fetchPayments(input: {
  runtimeBaseURL?: string;
  filters: PaymentFilters;
}): Promise<ListResponse<PaymentSummary>> {
  const query = toQuery({
    organization_id: input.filters.organization_id,
    customer_external_id: input.filters.customer_external_id,
    invoice_id: input.filters.invoice_id,
    invoice_number: input.filters.invoice_number,
    last_event_type: input.filters.last_event_type,
    payment_status: input.filters.payment_status,
    invoice_status: input.filters.invoice_status,
    payment_overdue: input.filters.payment_overdue,
    sort_by: input.filters.sort_by,
    order: input.filters.order,
    limit: input.filters.limit,
    offset: input.filters.offset,
  });

  const payload = await apiRequest<ListResponse<PaymentSummary>>(`/v1/payments${query}`, {
    runtimeBaseURL: input.runtimeBaseURL,
    method: "GET",
  });
  if (!payload) {
    throw new Error("unauthorized");
  }
  return payload;
}

export async function fetchPaymentDetail(input: {
  runtimeBaseURL?: string;
  paymentID: string;
}): Promise<PaymentDetail> {
  const payload = await apiRequest<PaymentDetail>(`/v1/payments/${encodeURIComponent(input.paymentID)}`, {
    runtimeBaseURL: input.runtimeBaseURL,
    method: "GET",
  });
  if (!payload) {
    throw new Error("unauthorized");
  }
  return payload;
}

export async function fetchPaymentEvents(input: {
  runtimeBaseURL?: string;
  paymentID: string;
  organizationID?: string;
  webhookType?: string;
  sortBy?: "received_at" | "occurred_at";
  order?: "asc" | "desc";
  limit?: number;
  offset?: number;
}): Promise<ListResponse<LagoWebhookEvent>> {
  const query = toQuery({
    organization_id: input.organizationID,
    webhook_type: input.webhookType,
    sort_by: input.sortBy,
    order: input.order,
    limit: input.limit,
    offset: input.offset,
  });

  const payload = await apiRequest<ListResponse<LagoWebhookEvent>>(
    `/v1/payments/${encodeURIComponent(input.paymentID)}/events${query}`,
    {
      runtimeBaseURL: input.runtimeBaseURL,
      method: "GET",
    }
  );
  if (!payload) {
    throw new Error("unauthorized");
  }
  return payload;
}

export async function retryPayment(input: {
  runtimeBaseURL?: string;
  paymentID: string;
  csrfToken: string;
}): Promise<Record<string, unknown>> {
  const payload = await apiRequest<Record<string, unknown>>(
    `/v1/payments/${encodeURIComponent(input.paymentID)}/retry`,
    {
      runtimeBaseURL: input.runtimeBaseURL,
      method: "POST",
      csrfToken: input.csrfToken,
      body: {},
    }
  );
  if (!payload) {
    throw new Error("unauthorized");
  }
  return payload;
}

export async function fetchInvoices(input: {
  runtimeBaseURL?: string;
  filters: InvoiceStatusFilters;
}): Promise<ListResponse<InvoiceSummary>> {
  const query = toQuery({
    organization_id: input.filters.organization_id,
    customer_external_id: input.filters.customer_external_id,
    payment_status: input.filters.payment_status,
    invoice_status: input.filters.invoice_status,
    payment_overdue: input.filters.payment_overdue,
    sort_by: input.filters.sort_by,
    order: input.filters.order,
    limit: input.filters.limit,
    offset: input.filters.offset,
  });

  const payload = await apiRequest<ListResponse<InvoiceSummary>>(`/v1/invoices${query}`, {
    runtimeBaseURL: input.runtimeBaseURL,
    method: "GET",
  });
  if (!payload) {
    throw new Error("unauthorized");
  }
  return payload;
}

export async function fetchInvoiceDetail(input: {
  runtimeBaseURL?: string;
  invoiceID: string;
}): Promise<InvoiceDetail> {
  const payload = await apiRequest<InvoiceDetail>(`/v1/invoices/${encodeURIComponent(input.invoiceID)}`, {
    runtimeBaseURL: input.runtimeBaseURL,
    method: "GET",
  });
  if (!payload) {
    throw new Error("unauthorized");
  }
  return payload;
}

export async function fetchInvoicePaymentReceipts(input: {
  runtimeBaseURL?: string;
  invoiceID: string;
}): Promise<PaymentReceiptSummary[]> {
  const payload = await apiRequest<{ items: PaymentReceiptSummary[] }>(
    `/v1/invoices/${encodeURIComponent(input.invoiceID)}/payment-receipts`,
    {
      runtimeBaseURL: input.runtimeBaseURL,
      method: "GET",
    }
  );
  if (!payload) {
    throw new Error("unauthorized");
  }
  return payload.items;
}

export async function fetchInvoiceCreditNotes(input: {
  runtimeBaseURL?: string;
  invoiceID: string;
}): Promise<CreditNoteSummary[]> {
  const payload = await apiRequest<{ items: CreditNoteSummary[] }>(
    `/v1/invoices/${encodeURIComponent(input.invoiceID)}/credit-notes`,
    {
      runtimeBaseURL: input.runtimeBaseURL,
      method: "GET",
    }
  );
  if (!payload) {
    throw new Error("unauthorized");
  }
  return payload.items;
}

export async function fetchInvoiceStatuses(input: {
  runtimeBaseURL?: string;
  filters: InvoiceStatusFilters;
}): Promise<ListResponse<InvoicePaymentStatusView>> {
  const query = toQuery({
    organization_id: input.filters.organization_id,
    payment_status: input.filters.payment_status,
    invoice_status: input.filters.invoice_status,
    payment_overdue: input.filters.payment_overdue,
    sort_by: input.filters.sort_by,
    order: input.filters.order,
    limit: input.filters.limit,
    offset: input.filters.offset,
  });

  const payload = await apiRequest<ListResponse<InvoicePaymentStatusView>>(
    `/v1/invoice-payment-statuses${query}`,
    {
      runtimeBaseURL: input.runtimeBaseURL,
      method: "GET",
    }
  );
  if (!payload) {
    throw new Error("unauthorized");
  }
  return payload;
}

export async function fetchInvoiceStatusSummary(input: {
  runtimeBaseURL?: string;
  organizationID?: string;
  staleAfterSec?: number;
}): Promise<InvoicePaymentStatusSummary> {
  const query = toQuery({
    organization_id: input.organizationID,
    stale_after_sec: input.staleAfterSec,
  });
  const payload = await apiRequest<InvoicePaymentStatusSummary>(
    `/v1/invoice-payment-statuses/summary${query}`,
    {
      runtimeBaseURL: input.runtimeBaseURL,
      method: "GET",
    }
  );
  if (!payload) {
    throw new Error("unauthorized");
  }
  return payload;
}

export async function fetchInvoiceEvents(input: {
  runtimeBaseURL?: string;
  invoiceID: string;
  organizationID?: string;
  webhookType?: string;
  sortBy?: "received_at" | "occurred_at";
  order?: "asc" | "desc";
  limit?: number;
  offset?: number;
}): Promise<ListResponse<LagoWebhookEvent>> {
  const query = toQuery({
    organization_id: input.organizationID,
    webhook_type: input.webhookType,
    sort_by: input.sortBy,
    order: input.order,
    limit: input.limit,
    offset: input.offset,
  });

  const payload = await apiRequest<ListResponse<LagoWebhookEvent>>(
    `/v1/invoice-payment-statuses/${encodeURIComponent(input.invoiceID)}/events${query}`,
    {
      runtimeBaseURL: input.runtimeBaseURL,
      method: "GET",
    }
  );
  if (!payload) {
    throw new Error("unauthorized");
  }
  return payload;
}

export async function fetchInvoiceLifecycle(input: {
  runtimeBaseURL?: string;
  invoiceID: string;
  eventLimit?: number;
}): Promise<InvoicePaymentLifecycle> {
  const query = toQuery({
    event_limit: input.eventLimit,
  });

  const payload = await apiRequest<InvoicePaymentLifecycle>(
    `/v1/invoice-payment-statuses/${encodeURIComponent(input.invoiceID)}/lifecycle${query}`,
    {
      runtimeBaseURL: input.runtimeBaseURL,
      method: "GET",
    }
  );
  if (!payload) {
    throw new Error("unauthorized");
  }
  return payload;
}

export async function retryInvoicePayment(input: {
  runtimeBaseURL?: string;
  invoiceID: string;
  csrfToken: string;
}): Promise<Record<string, unknown>> {
  const payload = await apiRequest<Record<string, unknown>>(
    `/v1/invoices/${encodeURIComponent(input.invoiceID)}/retry-payment`,
    {
      runtimeBaseURL: input.runtimeBaseURL,
      method: "POST",
      csrfToken: input.csrfToken,
      body: {},
    }
  );
  if (!payload) {
    throw new Error("unauthorized");
  }
  return payload;
}

export async function resendInvoiceEmail(input: {
  runtimeBaseURL?: string;
  invoiceID: string;
  csrfToken: string;
  to?: string[];
  cc?: string[];
  bcc?: string[];
}): Promise<NotificationDispatchResult> {
  const payload = await apiRequest<NotificationDispatchResult>(
    `/v1/invoices/${encodeURIComponent(input.invoiceID)}/resend-email`,
    {
      runtimeBaseURL: input.runtimeBaseURL,
      method: "POST",
      csrfToken: input.csrfToken,
      body: {
        to: input.to,
        cc: input.cc,
        bcc: input.bcc,
      },
    }
  );
  if (!payload) {
    throw new Error("unauthorized");
  }
  return payload;
}

export async function resendPaymentReceiptEmail(input: {
  runtimeBaseURL?: string;
  paymentReceiptID: string;
  csrfToken: string;
  to?: string[];
  cc?: string[];
  bcc?: string[];
}): Promise<NotificationDispatchResult> {
  const payload = await apiRequest<NotificationDispatchResult>(
    `/v1/payment-receipts/${encodeURIComponent(input.paymentReceiptID)}/resend-email`,
    {
      runtimeBaseURL: input.runtimeBaseURL,
      method: "POST",
      csrfToken: input.csrfToken,
      body: {
        to: input.to,
        cc: input.cc,
        bcc: input.bcc,
      },
    }
  );
  if (!payload) {
    throw new Error("unauthorized");
  }
  return payload;
}

export async function resendCreditNoteEmail(input: {
  runtimeBaseURL?: string;
  creditNoteID: string;
  csrfToken: string;
  to?: string[];
  cc?: string[];
  bcc?: string[];
}): Promise<NotificationDispatchResult> {
  const payload = await apiRequest<NotificationDispatchResult>(
    `/v1/credit-notes/${encodeURIComponent(input.creditNoteID)}/resend-email`,
    {
      runtimeBaseURL: input.runtimeBaseURL,
      method: "POST",
      csrfToken: input.csrfToken,
      body: {
        to: input.to,
        cc: input.cc,
        bcc: input.bcc,
      },
    }
  );
  if (!payload) {
    throw new Error("unauthorized");
  }
  return payload;
}

export async function fetchInvoiceExplainability(input: {
  runtimeBaseURL?: string;
  invoiceID: string;
  feeTypes?: string[];
  lineItemSort?: "created_at_asc" | "created_at_desc" | "amount_cents_asc" | "amount_cents_desc";
  page?: number;
  limit?: number;
}): Promise<InvoiceExplainability> {
  const params: Record<string, string | number | boolean | undefined> = {
    line_item_sort: input.lineItemSort,
    page: input.page,
    limit: input.limit,
  };
  if (input.feeTypes && input.feeTypes.length > 0) {
    params.fee_types = input.feeTypes.join(",");
  }
  const query = toQuery(params);

  const payload = await apiRequest<InvoiceExplainability>(
    `/v1/invoices/${encodeURIComponent(input.invoiceID)}/explainability${query}`,
    {
      runtimeBaseURL: input.runtimeBaseURL,
      method: "GET",
    }
  );
  if (!payload) {
    throw new Error("unauthorized");
  }
  return payload;
}

export async function fetchReplayJobs(input: {
  runtimeBaseURL?: string;
  customerID?: string;
  meterID?: string;
  status?: "queued" | "running" | "done" | "failed" | "";
  limit?: number;
  offset?: number;
  cursor?: string;
}): Promise<ListResponse<ReplayJob>> {
  const query = toQuery({
    customer_id: input.customerID,
    meter_id: input.meterID,
    status: input.status,
    limit: input.limit,
    offset: input.offset,
    cursor: input.cursor,
  });

  const payload = await apiRequest<ListResponse<ReplayJob>>(`/v1/replay-jobs${query}`, {
    runtimeBaseURL: input.runtimeBaseURL,
    method: "GET",
  });
  if (!payload) {
    throw new Error("unauthorized");
  }
  return payload;
}

export async function createReplayJob(input: {
  runtimeBaseURL?: string;
  csrfToken: string;
  customerID: string;
  meterID: string;
  from?: string;
  to?: string;
  idempotencyKey: string;
}): Promise<{ idempotent_replay: boolean; job: ReplayJob }> {
  const payload = await apiRequest<{ idempotent_replay: boolean; job: ReplayJob }>("/v1/replay-jobs", {
    runtimeBaseURL: input.runtimeBaseURL,
    method: "POST",
    csrfToken: input.csrfToken,
    body: {
      customer_id: input.customerID,
      meter_id: input.meterID,
      from: input.from || undefined,
      to: input.to || undefined,
      idempotency_key: input.idempotencyKey,
    },
  });
  if (!payload) {
    throw new Error("unauthorized");
  }
  return payload;
}

export async function fetchReplayJobDiagnostics(input: {
  runtimeBaseURL?: string;
  jobID: string;
}): Promise<ReplayJobDiagnostics> {
  const payload = await apiRequest<ReplayJobDiagnostics>(
    `/v1/replay-jobs/${encodeURIComponent(input.jobID)}/events`,
    {
      runtimeBaseURL: input.runtimeBaseURL,
      method: "GET",
    }
  );
  if (!payload) {
    throw new Error("unauthorized");
  }
  return payload;
}

export async function retryReplayJob(input: {
  runtimeBaseURL?: string;
  csrfToken: string;
  jobID: string;
}): Promise<ReplayJob> {
  const payload = await apiRequest<ReplayJob>(
    `/v1/replay-jobs/${encodeURIComponent(input.jobID)}/retry`,
    {
      runtimeBaseURL: input.runtimeBaseURL,
      method: "POST",
      csrfToken: input.csrfToken,
      body: {},
    }
  );
  if (!payload) {
    throw new Error("unauthorized");
  }
  return payload;
}
