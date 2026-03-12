import {
  InvoiceExplainability,
  InvoicePaymentStatusView,
  InvoiceStatusFilters,
  LagoWebhookEvent,
  ListResponse,
} from "@/lib/types";

function trimTrailingSlash(value: string): string {
  return value.replace(/\/+$/, "");
}

function resolveBaseURL(runtimeBaseURL?: string): string {
  const envBase = process.env.NEXT_PUBLIC_API_BASE_URL?.trim() ?? "";
  const candidate = runtimeBaseURL?.trim() || envBase;
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
    method?: "GET" | "POST";
    apiKey: string;
    runtimeBaseURL?: string;
    body?: unknown;
  }
): Promise<T> {
  const baseURL = resolveBaseURL(options.runtimeBaseURL);
  const endpoint = baseURL ? `${baseURL}${path}` : path;

  const response = await fetch(endpoint, {
    method: options.method ?? "GET",
    headers: {
      "Content-Type": "application/json",
      "X-API-Key": options.apiKey,
    },
    body: options.body === undefined ? undefined : JSON.stringify(options.body),
    cache: "no-store",
  });

  const isJSON = response.headers.get("content-type")?.includes("application/json");
  const payload = isJSON ? await response.json() : null;

  if (!response.ok) {
    const message =
      (payload && typeof payload.error === "string" && payload.error) ||
      `Request failed (${response.status})`;
    throw new Error(message);
  }

  return payload as T;
}

export async function fetchInvoiceStatuses(input: {
  apiKey: string;
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

  return apiRequest<ListResponse<InvoicePaymentStatusView>>(
    `/v1/invoice-payment-statuses${query}`,
    {
      apiKey: input.apiKey,
      runtimeBaseURL: input.runtimeBaseURL,
      method: "GET",
    }
  );
}

export async function fetchInvoiceEvents(input: {
  apiKey: string;
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

  return apiRequest<ListResponse<LagoWebhookEvent>>(
    `/v1/invoice-payment-statuses/${encodeURIComponent(input.invoiceID)}/events${query}`,
    {
      apiKey: input.apiKey,
      runtimeBaseURL: input.runtimeBaseURL,
      method: "GET",
    }
  );
}

export async function retryInvoicePayment(input: {
  apiKey: string;
  runtimeBaseURL?: string;
  invoiceID: string;
}): Promise<Record<string, unknown>> {
  return apiRequest<Record<string, unknown>>(
    `/v1/invoices/${encodeURIComponent(input.invoiceID)}/retry-payment`,
    {
      apiKey: input.apiKey,
      runtimeBaseURL: input.runtimeBaseURL,
      method: "POST",
      body: {},
    }
  );
}

export async function fetchInvoiceExplainability(input: {
  apiKey: string;
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

  return apiRequest<InvoiceExplainability>(
    `/v1/invoices/${encodeURIComponent(input.invoiceID)}/explainability${query}`,
    {
      apiKey: input.apiKey,
      runtimeBaseURL: input.runtimeBaseURL,
      method: "GET",
    }
  );
}
