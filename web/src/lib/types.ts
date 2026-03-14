export interface InvoicePaymentStatusView {
  tenant_id: string;
  organization_id: string;
  invoice_id: string;
  customer_external_id?: string;
  invoice_number?: string;
  currency?: string;
  invoice_status?: string;
  payment_status?: string;
  payment_overdue?: boolean;
  total_amount_cents?: number;
  total_due_amount_cents?: number;
  total_paid_amount_cents?: number;
  last_payment_error?: string;
  last_event_type: string;
  last_event_at: string;
  last_webhook_key: string;
  updated_at: string;
}

export interface UISession {
  authenticated: boolean;
  role: "reader" | "writer" | "admin";
  tenant_id: string;
  api_key_id: string;
  csrf_token: string;
  expires_at?: string;
}

export interface LagoWebhookEvent {
  id: string;
  tenant_id: string;
  organization_id: string;
  webhook_key: string;
  webhook_type: string;
  object_type: string;
  invoice_id?: string;
  payment_request_id?: string;
  dunning_campaign_code?: string;
  customer_external_id?: string;
  invoice_number?: string;
  currency?: string;
  invoice_status?: string;
  payment_status?: string;
  payment_overdue?: boolean;
  total_amount_cents?: number;
  total_due_amount_cents?: number;
  total_paid_amount_cents?: number;
  last_payment_error?: string;
  payload: Record<string, unknown>;
  received_at: string;
  occurred_at: string;
}

export interface ListResponse<T> {
  items: T[];
  total?: number;
  limit?: number;
  offset?: number;
  next_cursor?: string;
}

export interface InvoiceStatusFilters {
  organization_id?: string;
  payment_status?: string;
  invoice_status?: string;
  payment_overdue?: boolean;
  sort_by?: "last_event_at" | "updated_at" | "total_due_amount_cents" | "total_amount_cents";
  order?: "asc" | "desc";
  limit?: number;
  offset?: number;
}

export interface InvoicePaymentStatusSummary {
  total_invoices: number;
  overdue_count: number;
  attention_required_count: number;
  stale_attention_required: number;
  stale_after_seconds?: number;
  stale_before?: string;
  latest_event_at?: string;
  payment_status_counts: Record<string, number>;
  invoice_status_counts: Record<string, number>;
}

export interface InvoicePaymentLifecycle {
  tenant_id: string;
  organization_id: string;
  invoice_id: string;
  invoice_status?: string;
  payment_status?: string;
  payment_overdue?: boolean;
  last_payment_error?: string;
  last_event_type?: string;
  last_event_at?: string;
  updated_at?: string;
  events_analyzed: number;
  event_window_limit: number;
  event_window_truncated: boolean;
  distinct_webhook_types: string[];
  failure_event_count: number;
  success_event_count: number;
  pending_event_count: number;
  overdue_signal_count: number;
  last_failure_at?: string;
  last_success_at?: string;
  last_pending_at?: string;
  last_overdue_at?: string;
  requires_action: boolean;
  retry_recommended: boolean;
  recommended_action: "none" | "monitor_processing" | "retry_payment" | "collect_payment" | "investigate";
  recommended_action_note: string;
}

export interface InvoiceExplainabilityLineItem {
  fee_id: string;
  fee_type: string;
  item_name: string;
  item_code?: string;
  amount_cents: number;
  taxes_amount_cents: number;
  total_amount_cents: number;
  units?: number;
  events_count?: number;
  computation_mode: string;
  charge_model?: string;
  rule_reference: string;
  from_datetime?: string;
  to_datetime?: string;
  charge_filter_display_name?: string;
  subscription_id?: string;
  charge_id?: string;
  billable_metric_code?: string;
  properties: Record<string, unknown>;
}

export interface InvoiceExplainability {
  invoice_id: string;
  invoice_number: string;
  invoice_status: string;
  currency?: string;
  generated_at: string;
  total_amount_cents: number;
  explainability_version: string;
  explainability_digest: string;
  line_items_count: number;
  line_items: InvoiceExplainabilityLineItem[];
}

export interface ReplayJobWorkflowTelemetry {
  current_step: string;
  progress_percent: number;
  attempt_count: number;
  last_attempt_at?: string;
  processed_records: number;
  updated_at?: string;
}

export interface ReplayJobArtifactLinks {
  report_json?: string;
  report_csv?: string;
  dataset_digest?: string;
}

export interface ReplayJob {
  id: string;
  tenant_id?: string;
  customer_id?: string;
  meter_id?: string;
  from?: string;
  to?: string;
  idempotency_key: string;
  status: "queued" | "running" | "done" | "failed";
  attempt_count: number;
  last_attempt_at?: string;
  processed_records: number;
  error?: string;
  created_at: string;
  started_at?: string;
  completed_at?: string;
  workflow_telemetry?: ReplayJobWorkflowTelemetry;
  artifact_links?: ReplayJobArtifactLinks;
}

export interface ReplayJobDiagnostics {
  job: ReplayJob;
  usage_events_count: number;
  usage_quantity: number;
  billed_entries_count: number;
  billed_amount_cents: number;
}
