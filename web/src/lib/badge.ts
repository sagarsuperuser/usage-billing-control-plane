import type { StatusChipTone } from "@/components/ui/status-chip";

/**
 * Maps any status/state string to a StatusChip tone.
 * Centralizes the 14 duplicate tone/badge functions across screens.
 */
export function statusTone(status?: string): StatusChipTone {
  switch ((status || "").toLowerCase()) {
    case "active":
    case "ready":
    case "done":
    case "completed":
    case "succeeded":
    case "paid":
    case "connected":
    case "verified":
      return "success";

    case "pending":
    case "incomplete":
    case "queued":
    case "processing":
    case "draft":
    case "running":
    case "awaiting_payment_setup":
    case "pending_payment_setup":
      return "warning";

    case "failed":
    case "error":
    case "sync_error":
    case "revoked":
    case "overdue":
    case "voided":
    case "suspended":
    case "escalated":
    case "action_required":
      return "danger";

    case "paused":
    case "requires_action":
    case "retry_due":
      return "info";

    default:
      return "neutral";
  }
}

export function diagnosisTone(tone?: "healthy" | "warning" | "danger"): StatusChipTone {
  switch (tone) {
    case "healthy": return "success";
    case "warning": return "warning";
    case "danger": return "danger";
    default: return "neutral";
  }
}
