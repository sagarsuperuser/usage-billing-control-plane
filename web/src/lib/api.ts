import {
  Customer,
  CustomerOnboardingResult,
  CustomerReadiness,
  InvoiceExplainability,
  InvoicePaymentLifecycle,
  InvoicePaymentStatusView,
  InvoicePaymentStatusSummary,
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
  UISession,
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

export async function loginUISession(input: {
  apiKey: string;
  runtimeBaseURL?: string;
}): Promise<UISession> {
  const payload = await apiRequest<UISession>("/v1/ui/sessions/login", {
    method: "POST",
    runtimeBaseURL: input.runtimeBaseURL,
    body: { api_key: input.apiKey },
  });
  if (!payload) {
    throw new Error("login failed");
  }
  return payload;
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
