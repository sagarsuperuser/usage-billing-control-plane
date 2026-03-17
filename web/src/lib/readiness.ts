export function formatReadinessStatus(status?: string): string {
  switch ((status || "").toLowerCase()) {
    case "ready":
      return "Ready";
    case "pending":
      return "Needs attention";
    case "missing":
      return "Missing";
    case "incomplete":
      return "Incomplete";
    case "sync_error":
      return "Sync error";
    case "error":
      return "Error";
    case "active":
      return "Active";
    case "pending_payment_setup":
      return "Pending payment setup";
    case "action_required":
      return "Action required";
    case "suspended":
      return "Suspended";
    case "deleted":
      return "Deleted";
    case "archived":
      return "Archived";
    default:
      return status || "Unknown";
  }
}

const tenantSectionStepLabels: Record<string, string> = {
  tenant_exists: "Workspace record has not been created yet",
  tenant_active: "Workspace is not active yet",
  tenant_admin_ready: "Admin access has not been created yet",
  tenant_admin_key: "Admin access has not been created yet",
  billing_mapping_ready: "Billing connection is not configured yet",
  billing_mapping: "Billing connection is not configured yet",
  pricing: "Pricing rules still need to be configured",
  customer_created: "No billing-ready customer has been created yet",
};

const tenantStepLabels: Record<string, string> = {
  "tenant.tenant_exists": tenantSectionStepLabels.tenant_exists,
  "tenant.tenant_active": tenantSectionStepLabels.tenant_active,
  "tenant.tenant_admin_ready": tenantSectionStepLabels.tenant_admin_ready,
  "tenant.tenant_admin_key": tenantSectionStepLabels.tenant_admin_key,
  "billing_integration.billing_mapping_ready": tenantSectionStepLabels.billing_mapping_ready,
  "billing_integration.billing_mapping": tenantSectionStepLabels.billing_mapping,
  "billing_integration.pricing": tenantSectionStepLabels.pricing,
  "first_customer.customer_created": tenantSectionStepLabels.customer_created,
};

const customerStepLabels: Record<string, string> = {
  customer_exists: "Customer record has not been created yet",
  customer_active: "Customer is not active yet",
  billing_provider_configured: "Billing connection is not configured yet",
  lago_customer_synced: "Billing profile has not synced yet",
  payment_setup_ready: "Customer has not completed payment setup",
  default_payment_method_verified: "Default payment method has not been verified yet",
};

export function describeTenantMissingStep(step: string): string {
  return tenantStepLabels[step] || step.replaceAll("_", " ");
}

export function describeTenantSectionStep(step: string): string {
  return tenantSectionStepLabels[step] || describeTenantMissingStep(step);
}

export function describeCustomerMissingStep(step: string): string {
  return customerStepLabels[step] || step.replaceAll("_", " ");
}
