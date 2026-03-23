import type { DunningRun } from "@/lib/types";

export type DunningRunDiagnosis = {
  title: string;
  summary: string;
  nextStep: string;
  tone: "healthy" | "warning" | "danger";
};

export function dunningDiagnosisToneClass(tone: DunningRunDiagnosis["tone"]): string {
  switch (tone) {
    case "healthy":
      return "border-emerald-200 bg-emerald-50 text-emerald-800";
    case "warning":
      return "border-amber-200 bg-amber-50 text-amber-800";
    default:
      return "border-rose-200 bg-rose-50 text-rose-800";
  }
}

export function diagnoseDunningRun(run: DunningRun): DunningRunDiagnosis {
  if (run.paused) {
    return {
      title: "Run is paused",
      summary: "Scheduled retries and reminders are currently stopped for this invoice workflow.",
      nextStep: "Resume or resolve this run before expecting retries or reminders to continue.",
      tone: "warning",
    };
  }
  if (run.resolved_at) {
    return {
      title: "Run resolved",
      summary: "Collections no longer require operator action unless you are auditing the resolution trail.",
      nextStep: "Monitor only. Open run detail if you need the exact resolution trail.",
      tone: "healthy",
    };
  }
  switch (run.state) {
    case "awaiting_payment_setup":
      return {
        title: "Awaiting payment setup",
        summary: "Collection is blocked until the customer has a usable payment method path again.",
        nextStep: "Collect or refresh customer payment setup before expecting retry success.",
        tone: "danger",
      };
    case "retry_due":
      return {
        title: "Retry is due",
        summary: "The next collection attempt is ready or near due and should be checked against the invoice timeline.",
        nextStep: "Open the run and invoice timeline before manually retrying or overriding schedule.",
        tone: "warning",
      };
    case "escalated":
      return {
        title: "Manual review required",
        summary: "Automated collections have reached an escalation point and now require an operator decision.",
        nextStep: "Open run detail and decide whether to pause, resolve, or move the invoice into deeper recovery.",
        tone: "danger",
      };
    default:
      if (run.next_action_type === "collect_payment_reminder") {
        return {
          title: "Reminder path active",
          summary: "The workflow is currently centered on nudging the customer back into payment setup or collection readiness.",
          nextStep: "Confirm the reminder goes out and that the customer can complete payment setup.",
          tone: "warning",
        };
      }
      return {
        title: "Collections active",
        summary: "The workflow is progressing without a current operator block.",
        nextStep: "Monitor the next action timing and open the run if the state stops progressing.",
        tone: "healthy",
      };
  }
}
