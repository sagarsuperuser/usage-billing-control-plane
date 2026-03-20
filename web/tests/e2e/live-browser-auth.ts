import type { Page } from "@playwright/test";
import { expect } from "@playwright/test";

function escapeRegExp(value: string) {
  return value.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
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

  await expect(page.getByTestId("session-logout")).toBeVisible();
}
