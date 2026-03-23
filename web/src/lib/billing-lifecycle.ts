import type { DunningSummary, InvoicePaymentLifecycle } from "@/lib/types";

export function formatBillingState(value?: string): string {
  if (!value) return "-";
  return value.replaceAll("_", " ").replaceAll(".", " ");
}

type BillingLifecycleInput = {
  payment_status?: string;
  invoice_status?: string;
  payment_overdue?: boolean;
  last_payment_error?: string;
  last_event_type?: string;
  lifecycle?: Pick<
    InvoicePaymentLifecycle,
    | "recommended_action"
    | "recommended_action_note"
    | "last_event_type"
    | "failure_event_count"
    | "pending_event_count"
    | "overdue_signal_count"
    | "last_failure_at"
    | "last_pending_at"
    | "last_overdue_at"
  >;
  dunning?: Pick<
    DunningSummary,
    | "state"
    | "next_action_type"
    | "paused"
    | "attempt_count"
    | "last_notification_error"
    | "last_notification_status"
  >;
};

export type BillingDiagnosis = {
  code: string;
  tone: "healthy" | "warning" | "danger";
  title: string;
  summary: string;
  nextStep: string;
};

export type BillingEvidenceItem = {
  label: string;
  value: string;
};

export function billingFailureDiagnosis(subject: BillingLifecycleInput): BillingDiagnosis {
  const action = subject.lifecycle?.recommended_action;

  if (subject.dunning?.paused) {
    return {
      code: "dunning_paused",
      tone: "warning",
      title: "Collections workflow is paused",
      summary: "Automatic collection is not currently advancing because the dunning run is paused.",
      nextStep: "Resume or resolve the dunning run before expecting retries or reminders to continue.",
    };
  }

  if (subject.dunning?.last_notification_error) {
    return {
      code: "reminder_delivery_failed",
      tone: "warning",
      title: "Reminder delivery is failing",
      summary: "The collections workflow is active, but the most recent reminder dispatch failed.",
      nextStep: "Inspect the dunning run, fix the delivery issue, then resend the reminder if collection still depends on customer action.",
    };
  }

  switch (action) {
    case "collect_payment":
      return {
        code: "payment_collection_required",
        tone: "danger",
        title: "Payment collection is blocked",
        summary:
          subject.lifecycle?.recommended_action_note ||
          "The payment method or setup path is not ready enough for a clean retry.",
        nextStep: "Send or refresh payment setup, confirm the customer can complete it, then retry collection only after setup is ready.",
      };
    case "retry_payment":
      return {
        code: "retryable_failure",
        tone: "warning",
        title: "Failure looks retryable",
        summary:
          subject.lifecycle?.recommended_action_note ||
          "Recent failures look transient enough for a safe retry.",
        nextStep: "Retry payment first. Escalate to replay or explainability only if the state does not move afterward.",
      };
    case "investigate":
      return {
        code: "investigate_projection_or_state",
        tone: "danger",
        title: "State needs investigation",
        summary:
          subject.lifecycle?.recommended_action_note ||
          "The signal points to a projection, integration, or billing-state issue rather than a simple retryable failure.",
        nextStep: "Use the event timeline, explainability, and recovery tools before issuing another retry.",
      };
    case "monitor_processing":
      return {
        code: "processing_in_flight",
        tone: "warning",
        title: "Processing is still in flight",
        summary:
          subject.lifecycle?.recommended_action_note ||
          "Payment status is still moving and does not warrant manual intervention yet.",
        nextStep: "Monitor the event timeline and avoid manual retry until processing settles or becomes stale.",
      };
    default:
      if ((subject.payment_status || "").toLowerCase() === "failed" || subject.last_payment_error) {
        return {
          code: "payment_failed_no_guidance",
          tone: "danger",
          title: "Payment failed",
          summary: subject.last_payment_error || "A payment failure was recorded, but no stronger lifecycle recommendation is available yet.",
          nextStep: "Inspect the billing timeline for the last failure event, then decide whether to retry, collect payment, or escalate.",
        };
      }
      if (subject.payment_overdue) {
        return {
          code: "payment_overdue",
          tone: "warning",
          title: "Payment is overdue",
          summary: "The invoice is still unpaid past its due threshold.",
          nextStep: "Review recent events and dunning state before deciding between payment collection, reminder, or manual follow-up.",
        };
      }
      return {
        code: "healthy",
        tone: "healthy",
        title: "Billing state looks healthy",
        summary:
          subject.lifecycle?.recommended_action_note ||
          "No active payment failure or recovery signal currently requires operator action.",
        nextStep: "Use linked documents and timelines for normal inspection only.",
      };
  }
}

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

export function billingFailureEvidence(subject: BillingLifecycleInput): BillingEvidenceItem[] {
  const items: BillingEvidenceItem[] = [];

  if (subject.lifecycle?.recommended_action) {
    items.push({
      label: "Recommended action",
      value: formatBillingState(subject.lifecycle.recommended_action),
    });
  }

  const lastEvent = subject.last_event_type || subject.lifecycle?.last_event_type;
  if (lastEvent) {
    items.push({
      label: "Last event",
      value: formatBillingState(lastEvent),
    });
  }

  if (subject.last_payment_error) {
    items.push({
      label: "Last payment error",
      value: subject.last_payment_error,
    });
  }

  if (subject.dunning?.state) {
    items.push({
      label: "Dunning state",
      value: formatBillingState(subject.dunning.state),
    });
  }

  if (subject.dunning?.next_action_type) {
    items.push({
      label: "Dunning next action",
      value: formatBillingState(subject.dunning.next_action_type),
    });
  }

  if (subject.dunning?.last_notification_status) {
    items.push({
      label: "Reminder status",
      value: formatBillingState(subject.dunning.last_notification_status),
    });
  }

  if ((subject.lifecycle?.failure_event_count || 0) > 0) {
    items.push({
      label: "Failure signals",
      value: String(subject.lifecycle?.failure_event_count || 0),
    });
  }

  if ((subject.lifecycle?.pending_event_count || 0) > 0) {
    items.push({
      label: "Pending signals",
      value: String(subject.lifecycle?.pending_event_count || 0),
    });
  }

  if (subject.payment_overdue || (subject.lifecycle?.overdue_signal_count || 0) > 0) {
    items.push({
      label: "Overdue signals",
      value: String(subject.lifecycle?.overdue_signal_count || 0),
    });
  }

  if (subject.dunning?.paused) {
    items.push({
      label: "Collections workflow",
      value: "Paused",
    });
  }

  return items;
}
