import type { CustomerReadiness } from "@/lib/types";

export type CustomerCollectionDiagnosis = {
  code: string;
  tone: "healthy" | "warning" | "danger";
  title: string;
  summary: string;
  nextStep: string;
};

export function customerCollectionDiagnosisToneClass(tone: CustomerCollectionDiagnosis["tone"]): string {
  switch (tone) {
    case "healthy":
      return "border-emerald-200 bg-emerald-50 text-emerald-800";
    case "warning":
      return "border-amber-200 bg-amber-50 text-amber-800";
    default:
      return "border-rose-200 bg-rose-50 text-rose-800";
  }
}

export function diagnoseCustomerCollection(readiness: CustomerReadiness): CustomerCollectionDiagnosis {
  if (!readiness.customer_active) {
    return {
      code: "customer_inactive",
      tone: "danger",
      title: "Customer must be active first",
      summary: "Collection setup should not be treated as the primary blocker while the customer is still inactive.",
      nextStep: "Activate the customer first, then return here to complete payment collection setup.",
    };
  }

  if (readiness.billing_profile_status === "missing" || readiness.billing_profile_status === "incomplete") {
    return {
      code: "billing_profile_incomplete",
      tone: "danger",
      title: "Billing profile blocks collection",
      summary: "The customer still lacks the billing identity fields needed for a clean collection path.",
      nextStep: "Complete the billing profile first. Payment setup and invoice sync should not be treated as ready before that.",
    };
  }

  if (readiness.billing_profile_status === "sync_error" || !readiness.lago_customer_synced) {
    return {
      code: "billing_sync_error",
      tone: "warning",
      title: "Billing sync needs recovery",
      summary: "The billing profile is structurally ready, but the billing system mapping or sync state is stale.",
      nextStep: "Retry billing sync before assuming payment setup is the only problem.",
    };
  }

  if (readiness.payment_setup_status === "ready" && readiness.default_payment_method_verified) {
    return {
      code: "collection_ready",
      tone: "healthy",
      title: "Customer is collection-ready",
      summary: "Billing identity and payment setup are both ready enough for retry and normal collection flows.",
      nextStep: "Use invoice and payment surfaces for retry, monitoring, and dunning rather than re-running setup here.",
    };
  }

  if (readiness.payment_setup.last_request_status === "failed" || readiness.payment_setup_status === "error") {
    return {
      code: "setup_request_failed",
      tone: "warning",
      title: "Setup delivery needs attention",
      summary: "The customer still needs a payment method path, and the last setup delivery or verification attempt failed.",
      nextStep: "Resend the request or share a hosted setup link, then refresh verification before retrying collection.",
    };
  }

  if (readiness.payment_setup_status === "pending") {
    return {
      code: "awaiting_customer_setup",
      tone: "warning",
      title: "Awaiting customer payment setup",
      summary: "Collection is blocked until the customer completes the payment setup path and verification succeeds.",
      nextStep: "Use one clear setup path, then refresh verification here before retrying collection elsewhere.",
    };
  }

  return {
    code: "collection_missing",
    tone: "warning",
    title: "Payment setup still missing",
    summary: "The customer does not yet have a verified payment setup path.",
    nextStep: "Start the customer setup flow here before retrying collection from invoice or payment screens.",
  };
}
