import { defineConfig, devices } from "@playwright/test";

const port = Number(process.env.PLAYWRIGHT_WEB_PORT || 3100);
const liveBaseURL = process.env.PLAYWRIGHT_LIVE_BASE_URL || "";
const baseURL = liveBaseURL || process.env.PLAYWRIGHT_BASE_URL || `http://127.0.0.1:${port}`;
const useExternalBaseURL = liveBaseURL.length > 0;

export default defineConfig({
  testDir: "./tests/e2e",
  timeout: 45_000,
  expect: {
    timeout: 10_000,
  },
  fullyParallel: true,
  retries: process.env.CI ? 2 : 0,
  reporter: process.env.CI ? [["github"], ["html", { open: "never" }]] : [["list"]],
  use: {
    baseURL,
    trace: "retain-on-failure",
    screenshot: "only-on-failure",
    video: "retain-on-failure",
  },
  webServer: useExternalBaseURL
    ? undefined
    : {
        command: `npx -y pnpm@10.30.0 exec next dev --webpack --port ${port} --hostname 127.0.0.1`,
        url: baseURL,
        reuseExistingServer: !process.env.CI,
        timeout: 120_000,
      },
  projects: [
    {
      name: "chromium",
      use: { ...devices["Desktop Chrome"] },
    },
  ],
});
