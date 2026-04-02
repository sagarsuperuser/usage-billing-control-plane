import type { UISession } from "@/lib/types";

export function getDefaultLandingPath(session: UISession | null | undefined): string {
  if (session?.authenticated && session.scope === "platform") {
    return "/billing-connections";
  }
  if (session?.authenticated && session.scope === "tenant") {
    return "/customers";
  }
  return "/login";
}

export function normalizeNextPath(nextPath: string | null | undefined, fallbackPath: string): string {
  const candidate = (nextPath ?? "").trim();
  if (!candidate.startsWith("/") || candidate.startsWith("//")) {
    return fallbackPath;
  }
  if (candidate === "/" || candidate.startsWith("/login")) {
    return fallbackPath;
  }
  return candidate;
}

export function buildLoginPath(nextPath: string): string {
  const safeNext = normalizeNextPath(nextPath, "/control-plane");
  return `/login?next=${encodeURIComponent(safeNext)}`;
}

export function buildAccessSwitchPath(nextPath: string): string {
  const safeNext = normalizeNextPath(nextPath, "/control-plane");
  return `/login?switch=1&next=${encodeURIComponent(safeNext)}`;
}

export function buildWorkspaceSelectionPath(nextPath: string | null | undefined): string {
  const safeNext = normalizeNextPath(nextPath, "/customers");
  return `/workspace-select?next=${encodeURIComponent(safeNext)}`;
}
