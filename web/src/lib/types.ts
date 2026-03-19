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

export interface InvoiceSummary {
  invoice_id: string;
  invoice_number?: string;
  customer_external_id?: string;
  customer_display_name?: string;
  organization_id?: string;
  currency?: string;
  invoice_status?: string;
  payment_status?: string;
  payment_overdue?: boolean;
  total_amount_cents?: number;
  total_due_amount_cents?: number;
  total_paid_amount_cents?: number;
  last_payment_error?: string;
  issuing_date?: string;
  payment_due_date?: string;
  created_at?: string;
  updated_at?: string;
  last_event_at?: string;
}

export interface InvoiceDetail extends InvoiceSummary {
  lago_id?: string;
  billing_entity_code?: string;
  sequential_id?: unknown;
  invoice_type?: string;
  net_payment_term?: unknown;
  file_url?: string;
  xml_url?: string;
  version_number?: unknown;
  self_billed?: boolean;
  voided_at?: string;
  customer?: Record<string, unknown>;
  subscriptions?: unknown[];
  fees?: unknown[];
  metadata?: unknown[];
  applied_taxes?: unknown[];
}

export interface NotificationDispatchResult {
  dispatched_at: string;
  dispatched: boolean;
  action: string;
  domain: string;
  backend: string;
}


export interface PaymentSummary {
  invoice_id: string;
  invoice_number?: string;
  customer_external_id?: string;
  customer_display_name?: string;
  organization_id?: string;
  currency?: string;
  invoice_status?: string;
  payment_status?: string;
  payment_overdue?: boolean;
  total_amount_cents?: number;
  total_due_amount_cents?: number;
  total_paid_amount_cents?: number;
  last_payment_error?: string;
  last_event_type?: string;
  last_event_at?: string;
  updated_at?: string;
}

export interface PaymentDetail extends PaymentSummary {
  lifecycle: InvoicePaymentLifecycle;
}

export type PaymentFilters = InvoiceStatusFilters;

export interface UISession {
  authenticated: boolean;
  subject_type?: "user" | "api_key";
  subject_id?: string;
  user_email?: string;
  scope?: "tenant" | "platform";
  role?: "reader" | "writer" | "admin";
  platform_role?: "platform_admin";
  tenant_id?: string;
  api_key_id?: string;
  csrf_token: string;
  expires_at?: string;
}

export interface WorkspaceSelectionOption {
  tenant_id: string;
  name: string;
  role: "reader" | "writer" | "admin";
}

export interface WorkspaceSelectionState {
  required: boolean;
  user_email?: string;
  items: WorkspaceSelectionOption[];
  csrf_token?: string;
}

export interface UIAuthProvider {
  key: string;
  display_name: string;
  type: "oidc";
}

export interface UIAuthProviderList {
  password_enabled: boolean;
  password_reset_enabled: boolean;
  sso_providers: UIAuthProvider[];
}

export interface Tenant {
  id: string;
  name: string;
  status: "active" | "suspended" | "deleted";
  billing_provider_connection_id?: string;
  workspace_billing: WorkspaceBilling;
  created_at: string;
  updated_at: string;
}

export interface WorkspaceBilling {
  configured: boolean;
  connected: boolean;
  active_billing_connection_id?: string;
  status: string;
  source?: string;
  isolation_mode?: "shared" | "dedicated";
}

export interface WorkspaceMember {
  user_id: string;
  email: string;
  display_name: string;
  role: "reader" | "writer" | "admin";
  status: "active" | "disabled";
  platform_role?: "platform_admin";
  created_at: string;
  updated_at: string;
}

export interface WorkspaceInvitation {
  id: string;
  workspace_id: string;
  email: string;
  role: "reader" | "writer" | "admin";
  status: "pending" | "accepted" | "expired" | "revoked";
  expires_at: string;
  accepted_at?: string;
  accepted_by_user_id?: string;
  invited_by_user_id?: string;
  invited_by_platform_user: boolean;
  revoked_at?: string;
  created_at: string;
  updated_at: string;
  accept_url?: string;
}

export interface WorkspaceInvitationIssueResult {
  invitation: WorkspaceInvitation;
  accept_url: string;
  accept_path: string;
}

export interface ServiceAccount {
  id: string;
  workspace_id: string;
  name: string;
  description?: string;
  role: "reader" | "writer" | "admin";
  status: "active" | "disabled";
  purpose?: string;
  environment?: string;
  created_by_user_id?: string;
  created_by_platform_user?: boolean;
  created_at: string;
  updated_at: string;
  disabled_at?: string;
  active_credential_count: number;
  credentials: APIKey[];
}

export interface ServiceAccountCredentialIssueResult {
  service_account?: ServiceAccount;
  credential: APIKey;
  secret: string;
}

export interface APIKeyAuditEvent {
  id: string;
  tenant_id: string;
  api_key_id: string;
  actor_api_key_id?: string;
  action: string;
  metadata?: Record<string, unknown>;
  created_at: string;
}

export interface APIKeyAuditExportJobFilters {
  api_key_id?: string;
  actor_api_key_id?: string;
  action?: string;
  owner_type?: string;
  owner_id?: string;
}

export interface APIKeyAuditExportJob {
  id: string;
  tenant_id: string;
  requested_by_api_key_id?: string;
  idempotency_key: string;
  status: "queued" | "running" | "done" | "failed";
  filters: APIKeyAuditExportJobFilters;
  object_key?: string;
  row_count: number;
  error?: string;
  attempt_count: number;
  created_at: string;
  started_at?: string;
  completed_at?: string;
  expires_at?: string;
}

export interface APIKeyAuditExportJobResponse {
  job: APIKeyAuditExportJob;
  download_url?: string;
}

export interface WorkspaceInvitationPreview {
  invitation: WorkspaceInvitation;
  workspace_name: string;
  requires_login: boolean;
  authenticated: boolean;
  current_user_email?: string;
  email_matches_session: boolean;
  can_accept: boolean;
  account_exists: boolean;
}

export interface BillingProviderConnection {
  id: string;
  provider_type: "stripe";
  environment: "test" | "live";
  display_name: string;
  scope: "platform" | "tenant";
  owner_tenant_id?: string;
  status: "pending" | "connected" | "sync_error" | "disabled";
  workspace_ready: boolean;
  sync_state: "healthy" | "failed" | "never_synced" | "pending" | "disabled";
  sync_summary: string;
  linked_workspace_count: number;
  lago_organization_id?: string;
  lago_provider_code?: string;
  secret_configured: boolean;
  last_synced_at?: string;
  last_sync_error?: string;
  connected_at?: string;
  disabled_at?: string;
  created_by_type: string;
  created_by_id?: string;
  created_at: string;
  updated_at: string;
}

export interface PricingMetric {
  id: string;
  tenant_id?: string;
  key: string;
  name: string;
  unit: string;
  aggregation: string;
  rating_rule_version_id: string;
  created_at: string;
  updated_at: string;
}

export interface Plan {
  id: string;
  tenant_id?: string;
  code: string;
  name: string;
  description?: string;
  currency: string;
  billing_interval: "monthly" | "yearly";
  status: "draft" | "active" | "archived";
  base_amount_cents: number;
  meter_ids: string[];
  created_at: string;
  updated_at: string;
}

export interface SubscriptionSummary {
  id: string;
  tenant_id?: string;
  code: string;
  display_name: string;
  status: "draft" | "pending_payment_setup" | "active" | "action_required" | "archived";
  customer_id: string;
  customer_external_id: string;
  customer_display_name: string;
  plan_id: string;
  plan_code: string;
  plan_name: string;
  billing_interval: "monthly" | "yearly";
  currency: string;
  base_amount_cents: number;
  payment_setup_status: "missing" | "pending" | "ready" | "error";
  default_payment_method_verified: boolean;
  payment_setup_action_required: boolean;
  payment_setup_requested_at?: string;
  activated_at?: string;
  created_at: string;
  updated_at: string;
}

export interface SubscriptionDetail extends SubscriptionSummary {
  customer: Customer;
  plan: Plan;
  billing_profile: CustomerBillingProfile;
  payment_setup: CustomerPaymentSetup;
  missing_steps: string[];
}

export interface SubscriptionPaymentSetupResult {
  action: "requested" | "resent" | "already_ready";
  checkout_url?: string;
  subscription: SubscriptionDetail;
}

export interface CreateSubscriptionResult {
  subscription: SubscriptionDetail;
  payment_setup_started: boolean;
  checkout_url?: string;
}

export interface APIKey {
  id: string;
  key_prefix: string;
  name: string;
  role: string;
  tenant_id: string;
  owner_type?: string;
  owner_id?: string;
  purpose?: string;
  environment?: string;
  created_by_user_id?: string;
  created_by_platform_user?: boolean;
  created_at: string;
  expires_at?: string;
  revoked_at?: string;
  last_used_at?: string;
  last_rotated_at?: string;
  rotation_required_at?: string;
  revocation_reason?: string;
}

export interface TenantCoreReadiness {
  status: string;
  tenant_exists: boolean;
  tenant_active: boolean;
  tenant_admin_ready: boolean;
  missing_steps: string[];
}

export interface BillingIntegrationReadiness {
  status: string;
  billing_mapping_ready: boolean;
  billing_connected: boolean;
  workspace_billing_status?: string;
  workspace_billing_source?: string;
  active_billing_connection_id?: string;
  isolation_mode?: "shared" | "dedicated";
  pricing_ready: boolean;
  missing_steps: string[];
}

export interface FirstCustomerBillingReadiness {
  status: string;
  managed: boolean;
  customer_exists: boolean;
  customer_external_id?: string;
  customer_active: boolean;
  billing_profile_status: "missing" | "incomplete" | "ready" | "sync_error";
  payment_setup_status: "missing" | "pending" | "ready" | "error";
  missing_steps: string[];
  note?: string;
}

export interface TenantOnboardingReadiness {
  status: string;
  missing_steps: string[];
  tenant: TenantCoreReadiness;
  billing_integration: BillingIntegrationReadiness;
  first_customer: FirstCustomerBillingReadiness;
}

export interface TenantAdminBootstrapResult {
  created: boolean;
  existing_active_keys: number;
  service_account?: ServiceAccount;
  credential?: APIKey;
  secret?: string;
}

export interface TenantOnboardingResult {
  tenant: Tenant;
  tenant_created: boolean;
  tenant_admin_bootstrap: TenantAdminBootstrapResult;
  readiness: TenantOnboardingReadiness;
}

export interface Customer {
  id: string;
  tenant_id?: string;
  external_id: string;
  display_name: string;
  email?: string;
  status: "active" | "suspended" | "archived";
  lago_customer_id?: string;
  created_at: string;
  updated_at: string;
}

export interface CustomerBillingProfile {
  customer_id: string;
  tenant_id?: string;
  legal_name?: string;
  email?: string;
  phone?: string;
  billing_address_line1?: string;
  billing_address_line2?: string;
  billing_city?: string;
  billing_state?: string;
  billing_postal_code?: string;
  billing_country?: string;
  currency?: string;
  tax_identifier?: string;
  provider_code?: string;
  profile_status: "missing" | "incomplete" | "ready" | "sync_error";
  last_synced_at?: string;
  last_sync_error?: string;
  created_at: string;
  updated_at: string;
}

export interface CustomerPaymentSetup {
  customer_id: string;
  tenant_id?: string;
  setup_status: "missing" | "pending" | "ready" | "error";
  default_payment_method_present: boolean;
  payment_method_type?: string;
  provider_customer_reference?: string;
  provider_payment_method_reference?: string;
  last_verified_at?: string;
  last_verification_result?: string;
  last_verification_error?: string;
  created_at: string;
  updated_at: string;
}

export interface CustomerReadiness {
  status: string;
  missing_steps: string[];
  customer_exists: boolean;
  customer_active: boolean;
  billing_provider_configured: boolean;
  lago_customer_synced: boolean;
  default_payment_method_verified: boolean;
  billing_profile_status: "missing" | "incomplete" | "ready" | "sync_error";
  payment_setup_status: "missing" | "pending" | "ready" | "error";
  billing_profile: CustomerBillingProfile;
  payment_setup: CustomerPaymentSetup;
}

export interface CustomerOnboardingResult {
  customer: Customer;
  customer_created: boolean;
  billing_profile_applied: boolean;
  payment_setup_started: boolean;
  checkout_url?: string;
  billing_profile: CustomerBillingProfile;
  payment_setup: CustomerPaymentSetup;
  readiness: CustomerReadiness;
}

export interface RetryCustomerBillingProfileSyncResult {
  external_id: string;
  billing_profile: CustomerBillingProfile;
  payment_setup: CustomerPaymentSetup;
  readiness: CustomerReadiness;
}

export interface RefreshCustomerPaymentSetupResult {
  external_id: string;
  payment_setup: CustomerPaymentSetup;
  readiness: CustomerReadiness;
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
  customer_external_id?: string;
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
  error_json?: string;
  dataset_digest?: string;
}

export interface ReplayJob {
  id: string;
  tenant_id?: string;
  customer_id: string;
  meter_id?: string;
  from?: string;
  to?: string;
  status: string;
  attempt_count: number;
  last_attempt_at?: string;
  processed_records: number;
  error?: string;
  started_at?: string;
  completed_at?: string;
  idempotency_key?: string;
  created_at: string;
  updated_at: string;
  artifact_links?: ReplayJobArtifactLinks;
  workflow_telemetry?: ReplayJobWorkflowTelemetry;
}

export interface ReplayJobDiagnostics {
  job: ReplayJob;
  usage_events_count: number;
  usage_quantity: number;
  billed_entries_count: number;
  billed_amount_cents: number;
  events: Array<Record<string, unknown>>;
}
