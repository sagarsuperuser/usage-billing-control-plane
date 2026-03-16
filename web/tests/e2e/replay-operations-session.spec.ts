import { expect, test, type Page } from "@playwright/test";

type ReplayMockWindow = Window & typeof globalThis & {
  __replayMock: {
    createCSRF: string;
    retryCSRF: string;
    lastCreatedID: string;
  };
};

type SessionPayload = {
  authenticated: boolean;
  role: "reader" | "writer" | "admin";
  tenant_id: string;
  api_key_id: string;
  csrf_token: string;
};

type ReplayJob = {
  id: string;
  tenant_id: string;
  customer_id: string;
  meter_id: string;
  idempotency_key: string;
  status: "queued" | "running" | "done" | "failed";
  attempt_count: number;
  processed_records: number;
  error?: string;
  created_at: string;
  last_attempt_at?: string;
  workflow_telemetry: {
    current_step: string;
    progress_percent: number;
    attempt_count: number;
    processed_records: number;
    updated_at: string;
  };
  artifact_links: {
    report_json: string;
    report_csv: string;
    dataset_digest: string;
  };
};

const sessionPayload: SessionPayload = {
  authenticated: true,
  role: "writer",
  tenant_id: "tenant_a",
  api_key_id: "api_key_writer_1",
  csrf_token: "csrf-replay-123",
};

function buildJob(partial: Partial<ReplayJob> & Pick<ReplayJob, "id" | "customer_id" | "meter_id" | "idempotency_key" | "status">): ReplayJob {
  const now = new Date().toISOString();
  return {
    tenant_id: "tenant_a",
    attempt_count: 1,
    processed_records: 12,
    created_at: now,
    workflow_telemetry: {
      current_step: partial.status === "failed" ? "failed" : partial.status,
      progress_percent: partial.status === "done" ? 100 : partial.status === "failed" ? 100 : 50,
      attempt_count: 1,
      processed_records: 12,
      updated_at: now,
    },
    artifact_links: {
      report_json: `http://127.0.0.1/report/${partial.id}.json`,
      report_csv: `http://127.0.0.1/report/${partial.id}.csv`,
      dataset_digest: `http://127.0.0.1/report/${partial.id}.txt`,
    },
    ...partial,
  };
}

const initialJobs: ReplayJob[] = [
  buildJob({
    id: "job_failed",
    customer_id: "cust_failure",
    meter_id: "meter_alpha",
    idempotency_key: "replay-failed-1",
    status: "failed",
    error: "workflow activity failed",
  }),
  buildJob({
    id: "job_done",
    customer_id: "cust_done",
    meter_id: "meter_beta",
    idempotency_key: "replay-done-1",
    status: "done",
  }),
];

async function installReplayMock(page: Page, session: SessionPayload) {
  await page.addInitScript(({ session, jobsSeed }: { session: SessionPayload; jobsSeed: ReplayJob[] }) => {
    let loggedIn = true;
    let jobs = jobsSeed.map((job) => ({ ...job }));
    const w = window as ReplayMockWindow;
    w.__replayMock = {
      createCSRF: "",
      retryCSRF: "",
      lastCreatedID: "",
    };

    const json = (status: number, payload: unknown) =>
      new Response(JSON.stringify(payload), {
        status,
        headers: {
          "Content-Type": "application/json",
        },
      });

    const buildDiagnostics = (job: ReplayJob) => ({
      job,
      usage_events_count: job.id === "job_failed" ? 4 : 2,
      usage_quantity: job.id === "job_failed" ? 120 : 40,
      billed_entries_count: job.id === "job_failed" ? 1 : 2,
      billed_amount_cents: job.id === "job_failed" ? 1800 : 900,
    });

    const originalFetch = window.fetch.bind(window);

    window.fetch = async (input, init) => {
      const request = input instanceof Request ? input : null;
      const method = (init?.method || request?.method || "GET").toUpperCase();
      const rawURL =
        typeof input === "string"
          ? input
          : input instanceof URL
            ? input.toString()
            : input.url;
      const url = new URL(rawURL, window.location.origin);
      const path = url.pathname;
      const headers = new Headers(init?.headers || request?.headers);

      if (path === "/v1/ui/sessions/me" && method === "GET") {
        return loggedIn ? json(200, session) : json(401, { error: "unauthorized" });
      }

      if (path === "/v1/ui/sessions/login" && method === "POST") {
        loggedIn = true;
        return json(201, session);
      }

      if (path === "/v1/ui/sessions/logout" && method === "POST") {
        const csrf = headers.get("X-CSRF-Token") || "";
        if (csrf !== session.csrf_token) {
          return json(403, { error: "forbidden" });
        }
        loggedIn = false;
        return json(200, { logged_out: true });
      }

      if (path === "/v1/replay-jobs" && method === "GET") {
        return loggedIn ? json(200, { items: jobs, total: jobs.length, limit: 20, offset: 0 }) : json(401, { error: "unauthorized" });
      }

      if (path === "/v1/replay-jobs" && method === "POST") {
        const csrf = headers.get("X-CSRF-Token") || "";
        w.__replayMock.createCSRF = csrf;
        const body = JSON.parse(String(init?.body || "{}"));
        const id = "job_new";
        const createdAt = new Date().toISOString();
        const newJob: ReplayJob = {
          id,
          tenant_id: session.tenant_id,
          customer_id: body.customer_id,
          meter_id: body.meter_id,
          idempotency_key: body.idempotency_key,
          status: "queued",
          attempt_count: 0,
          processed_records: 0,
          created_at: createdAt,
          workflow_telemetry: {
            current_step: "queued",
            progress_percent: 0,
            attempt_count: 0,
            processed_records: 0,
            updated_at: createdAt,
          },
          artifact_links: {
            report_json: `http://127.0.0.1/report/${id}.json`,
            report_csv: `http://127.0.0.1/report/${id}.csv`,
            dataset_digest: `http://127.0.0.1/report/${id}.txt`,
          },
        };
        jobs = [newJob, ...jobs];
        w.__replayMock.lastCreatedID = id;
        return json(201, { idempotent_replay: false, job: newJob });
      }

      if (path.startsWith("/v1/replay-jobs/") && path.endsWith("/events") && method === "GET") {
        const jobID = path.split("/")[3];
        const job = jobs.find((item) => item.id === jobID);
        if (!job) {
          return json(404, { error: "not found" });
        }
        return json(200, buildDiagnostics(job));
      }

      if (path.startsWith("/v1/replay-jobs/") && path.endsWith("/retry") && method === "POST") {
        const jobID = path.split("/")[3];
        const csrf = headers.get("X-CSRF-Token") || "";
        w.__replayMock.retryCSRF = csrf;
        jobs = jobs.map((job) =>
          job.id === jobID
            ? {
                ...job,
                status: "queued",
                error: "",
                attempt_count: job.attempt_count + 1,
                workflow_telemetry: {
                  ...job.workflow_telemetry,
                  current_step: "queued",
                  progress_percent: 0,
                  attempt_count: job.attempt_count + 1,
                  updated_at: new Date().toISOString(),
                },
              }
            : job
        );
        const job = jobs.find((item) => item.id === jobID);
        return json(200, job);
      }

      return originalFetch(input, init);
    };
  }, { session, jobsSeed: initialJobs });
}

test.beforeEach(async ({ page }) => {
  await installReplayMock(page, sessionPayload);
});

test("writer session can queue replay jobs and inspect diagnostics", async ({ page }) => {
  await page.goto("/replay-operations");

  await expect(page.getByText("Replay + Reprocess Operations")).toBeVisible();
  await page.getByTestId("replay-create-customer-id").fill("cust_new");
  await page.getByTestId("replay-create-meter-id").fill("meter_new");
  await page.getByTestId("replay-create-idempotency-key").fill("replay-new-1");
  await page.getByTestId("replay-create-submit").click();

  await expect.poll(async () => page.evaluate(() => (window as ReplayMockWindow).__replayMock.createCSRF)).toBe("csrf-replay-123");
  await expect(page.getByTestId("replay-flash-message")).toContainText("Replay job job_new queued");
  await expect(page.getByTestId("replay-diagnostics-drawer")).toBeVisible();
  await expect(page.getByText("Usage events")).toBeVisible();
  await expect(page.getByTestId("replay-job-row-job_new")).toBeVisible();
});

test("writer session can retry failed replay jobs with csrf", async ({ page }) => {
  await page.goto("/replay-operations");

  await expect(page.getByTestId("replay-job-row-job_failed")).toBeVisible();
  await page.getByTestId("replay-open-diagnostics-job_failed").click();
  await expect(page.getByTestId("replay-diagnostics-drawer")).toBeVisible();

  await page.getByTestId("replay-diagnostics-retry").click();

  await expect.poll(async () => page.evaluate(() => (window as ReplayMockWindow).__replayMock.retryCSRF)).toBe("csrf-replay-123");
  await expect(page.getByTestId("replay-flash-message")).toContainText("re-queued for recovery");
  await expect(page.getByTestId("replay-job-row-job_failed")).toContainText("queued");
});

test("reader session is read-only for replay queue and retry actions", async ({ page }) => {
  await installReplayMock(page, {
    ...sessionPayload,
    role: "reader",
    api_key_id: "api_key_reader_1",
  });

  await page.goto("/replay-operations");

  await expect(page.getByTestId("replay-read-only-notice")).toContainText("read-only for replay queue and recovery actions");
  await expect(page.getByTestId("replay-create-submit")).toBeDisabled();
  await expect(page.getByTestId("replay-retry-job-job_failed")).toBeDisabled();

  await page.getByTestId("replay-open-diagnostics-job_failed").click();
  await expect(page.getByTestId("replay-diagnostics-drawer")).toBeVisible();
  await expect(page.getByTestId("replay-diagnostics-retry")).toBeDisabled();
});
