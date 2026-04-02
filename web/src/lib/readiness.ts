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
  tenant_exists: "Create the workspace to continue",
  tenant_active: "Activate the workspace to continue",
  tenant_admin_ready: "Create an admin account for this workspace",
  tenant_admin_key: "Create an admin account for this workspace",
  billing_mapping_ready: "Attach a billing connection to this workspace",
  billing_mapping: "Attach a billing connection to this workspace",
  billing_verification: "Verify the billing connection — check credentials and retry sync",
  pricing: "Create at least one metric and plan before going live",
  customer_created: "Add the first customer to complete workspace setup",
};

const tenantStepLabels: Record<string, string> = {
  "tenant.tenant_exists": tenantSectionStepLabels.tenant_exists,
  "tenant.tenant_active": tenantSectionStepLabels.tenant_active,
  "tenant.tenant_admin_ready": tenantSectionStepLabels.tenant_admin_ready,
  "tenant.tenant_admin_key": tenantSectionStepLabels.tenant_admin_key,
  "billing_integration.billing_mapping_ready": tenantSectionStepLabels.billing_mapping_ready,
  "billing_integration.billing_mapping": tenantSectionStepLabels.billing_mapping,
  "billing_integration.billing_verification": tenantSectionStepLabels.billing_verification,
  "billing_integration.pricing": tenantSectionStepLabels.pricing,
  "first_customer.customer_created": tenantSectionStepLabels.customer_created,
};

const customerStepLabels: Record<string, string> = {
  customer_exists: "Create the customer record first",
  customer_active: "Activate the customer to continue",
  billing_provider_configured: "Attach a billing connection to this workspace",
  lago_customer_synced: "Complete the billing profile — legal name, address, and currency are required",
  payment_setup_ready: "Send a payment setup request so the customer can add a payment method",
  default_payment_method_verified: "Refresh payment setup after the customer completes checkout",
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

export function normalizeMissingSteps(steps?: string[] | null): string[] {
  return Array.isArray(steps)
    ? steps.filter((step): step is string => typeof step === "string" && step.trim().length > 0)
    : [];
}
