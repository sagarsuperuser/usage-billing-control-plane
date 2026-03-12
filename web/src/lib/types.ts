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
  limit?: number;
  offset?: number;
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
