import { formatDistanceToNowStrict } from "date-fns";

export function formatMoney(cents?: number, currency = "USD"): string {
  if (typeof cents !== "number") return "-";
  return new Intl.NumberFormat("en-US", {
    style: "currency",
    currency,
    minimumFractionDigits: 2,
  }).format(cents / 100);
}

export function formatRelativeTimestamp(value?: string): string {
  if (!value) return "-";
  const date = new Date(value);
  if (Number.isNaN(date.valueOf())) return "-";
  return `${formatDistanceToNowStrict(date, { addSuffix: true })}`;
}

export function formatExactTimestamp(value?: string): string {
  if (!value) return "-";
  const date = new Date(value);
  if (Number.isNaN(date.valueOf())) return "-";
  return date.toLocaleString();
}
