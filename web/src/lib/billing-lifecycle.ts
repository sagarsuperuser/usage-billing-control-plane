import type { InvoicePaymentLifecycle } from "@/lib/types";

export function formatBillingState(value?: string): string {
  if (!value) return "-";
  return value.replaceAll("_", " ").replaceAll(".", " ");
}

type BillingLifecycleInput = {
  lifecycle?: Pick<InvoicePaymentLifecycle, "recommended_action" | "recommended_action_note">;
};

export function billingActionConfig(subject: BillingLifecycleInput) {
  switch (subject.lifecycle?.recommended_action) {
    case "retry_payment":
      return {
        title: "Retry payment collection",
        body:
          subject.lifecycle.recommended_action_note ||
          "Recent failures look retryable. Retry collection first, then escalate to recovery or explainability only if the state does not move.",
        emphasizeRetry: true,
        showRecovery: false,
        showExplainability: false,
      };
    case "collect_payment":
      return {
        title: "Collect payment before retrying",
        body:
          subject.lifecycle.recommended_action_note ||
          "The main issue is missing or incomplete payment collection. Use customer and invoice flows to collect payment before running deeper recovery.",
        emphasizeRetry: false,
        showRecovery: false,
        showExplainability: false,
      };
    case "investigate":
      return {
        title: "Investigate before retrying",
        body:
          subject.lifecycle.recommended_action_note ||
          "The signal points to a state or projection issue rather than a simple transient failure. Use explainability and replay recovery before retrying collection.",
        emphasizeRetry: false,
        showRecovery: true,
        showExplainability: true,
      };
    case "monitor_processing":
      return {
        title: "Monitor processing state",
        body:
          subject.lifecycle.recommended_action_note ||
          "Payment processing is still in flight. Monitor the event timeline before taking manual recovery action.",
        emphasizeRetry: false,
        showRecovery: false,
        showExplainability: false,
      };
    default:
      return {
        title: "No action required",
        body:
          subject.lifecycle?.recommended_action_note ||
          "Payment activity currently looks healthy. Use linked invoice, customer, and event timelines for normal inspection.",
        emphasizeRetry: false,
        showRecovery: false,
        showExplainability: false,
      };
  }
}
