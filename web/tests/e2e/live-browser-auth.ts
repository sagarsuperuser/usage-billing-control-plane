import type { Page } from "@playwright/test";
import { expect } from "@playwright/test";

function escapeRegExp(value: string) {
  return value.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
}

async function waitForAuthenticatedSession(page: Page) {
  await expect
    .poll(
      async () =>
        page.evaluate(async () => {
          const runtimeResponse = await fetch("/runtime-config", {
            method: "GET",
            cache: "no-store",
            credentials: "same-origin",
          });
          if (!runtimeResponse.ok) {
            return { authenticated: false, status: runtimeResponse.status, source: "runtime-config" };
          }
          const runtimePayload = (await runtimeResponse.json()) as { apiBaseURL?: string } | null;
          const baseURL = (runtimePayload?.apiBaseURL ?? window.location.origin).replace(/\/+$/, "");
          const sessionResponse = await fetch(`${baseURL}/v1/ui/sessions/me`, {
            method: "GET",
            cache: "no-store",
            credentials: "include",
          });
          if (!sessionResponse.ok) {
            return { authenticated: false, status: sessionResponse.status, source: "ui-session" };
          }
          const payload = (await sessionResponse.json()) as { authenticated?: boolean; scope?: string } | null;
          return {
            authenticated: payload?.authenticated === true,
            status: sessionResponse.status,
            scope: payload?.scope ?? null,
            source: "ui-session",
          };
        }),
      { timeout: 15000 },
    )
    .toMatchObject({ authenticated: true, source: "ui-session" });
}

export async function loginWithPassword(page: Page, options: { email: string; password: string; nextPath: string }) {
  const nextPath = options.nextPath.startsWith("/") ? options.nextPath : `/${options.nextPath}`;
  await page.goto(`/login?next=${encodeURIComponent(nextPath)}`);

  await expect(page.getByTestId("session-login-email")).toBeVisible();
  await page.getByTestId("session-login-email").fill(options.email);
  await page.getByTestId("session-login-password").fill(options.password);

  await Promise.all([
    page.waitForURL(new RegExp(`${escapeRegExp(nextPath)}(?:\\?.*)?$`), { timeout: 15000 }),
    page.getByTestId("session-login-submit").click(),
  ]);

  await waitForAuthenticatedSession(page);
  await expect(page.getByTestId("session-login-email")).toHaveCount(0);
}
