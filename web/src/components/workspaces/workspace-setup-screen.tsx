import { useState } from "react";
import { Building2, LoaderCircle } from "lucide-react";

import { createWorkspace } from "@/lib/api";
import { useUISession } from "@/hooks/use-ui-session";

export function WorkspaceSetupScreen() {
  const { apiBaseURL, csrfToken, isAuthenticated, setSession } = useUISession();
  const [name, setName] = useState("");
  const [error, setError] = useState("");
  const [submitting, setSubmitting] = useState(false);

  const onSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    const trimmed = name.trim();
    if (!trimmed) return;
    setError("");
    setSubmitting(true);
    try {
      await createWorkspace({ runtimeBaseURL: apiBaseURL, csrfToken, name: trimmed });
      setSession(null);
      window.location.assign("/control-plane");
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to create workspace");
      setSubmitting(false);
    }
  };

  if (!isAuthenticated) {
    return (
      <div className="flex min-h-screen items-center justify-center bg-background px-4">
        <div className="rounded-xl border border-border bg-surface px-6 py-4 text-sm text-text-muted shadow-sm">
          <a href="/login" className="font-medium text-text-secondary hover:text-text-primary">Sign in</a> to create a workspace.
        </div>
      </div>
    );
  }

  return (
    <div className="flex min-h-screen items-center justify-center bg-background px-4">
      <div className="w-full max-w-[400px]">
        <div className="flex items-center gap-3">
          <div className="flex h-10 w-10 items-center justify-center rounded-xl bg-slate-900">
            <Building2 className="h-5 w-5 text-white" />
          </div>
          <div>
            <h1 className="text-lg font-semibold text-text-primary">New workspace</h1>
            <p className="text-xs text-text-muted">Each workspace has its own billing, API keys, and team.</p>
          </div>
        </div>

        <form onSubmit={onSubmit} className="mt-6">
          <label className="mb-1.5 block text-xs font-semibold uppercase tracking-wider text-text-muted">
            Workspace name
          </label>
          <input
            type="text"
            value={name}
            onChange={(e) => setName(e.target.value)}
            placeholder="e.g. Acme Corp"
            autoFocus
            className="h-11 w-full rounded-xl border border-border bg-surface px-3.5 text-sm text-text-primary outline-none ring-slate-300 transition placeholder:text-text-faint focus:ring-2"
          />

          <button
            type="submit"
            disabled={!name.trim() || !csrfToken || submitting}
            className="mt-4 inline-flex h-11 w-full items-center justify-center gap-2 rounded-xl bg-slate-900 text-sm font-semibold text-white transition hover:bg-slate-800 disabled:opacity-50"
          >
            {submitting ? <LoaderCircle className="h-4 w-4 animate-spin" /> : <Building2 className="h-4 w-4" />}
            Create workspace
          </button>

          {error ? <p className="mt-3 text-xs text-rose-600">{error}</p> : null}
        </form>

        <p className="mt-6 text-center text-xs text-text-faint">
          <a href="/control-plane" className="font-medium text-text-muted hover:text-text-secondary">Back to dashboard</a>
        </p>
      </div>
    </div>
  );
}
