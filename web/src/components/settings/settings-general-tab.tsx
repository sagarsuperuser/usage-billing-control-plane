
import { CheckCircle2, Copy, LoaderCircle } from "lucide-react";
import { useEffect, useState } from "react";
import { useForm } from "react-hook-form";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import { fetchWorkspaceSettings, updateWorkspaceSettings, updateUserProfile } from "@/lib/api";
import { showError, showSuccess } from "@/lib/toast";
import type { UISession } from "@/lib/types";

export function SettingsGeneralTab({
  apiBaseURL,
  csrfToken,
  session,
}: {
  apiBaseURL: string;
  csrfToken: string;
  session: UISession | null;
}) {
  const queryClient = useQueryClient();
  const queryKey = ["workspace-settings", apiBaseURL];

  const settingsQuery = useQuery({
    queryKey,
    queryFn: () => fetchWorkspaceSettings({ runtimeBaseURL: apiBaseURL }),
    staleTime: 30_000,
  });

  const workspace = settingsQuery.data?.workspace;
  const [copied, setCopied] = useState(false);

  const {
    register,
    handleSubmit,
    reset,
    formState: { isDirty, isSubmitting },
  } = useForm<{ name: string }>({ defaultValues: { name: "" } });

  useEffect(() => {
    if (workspace?.name) reset({ name: workspace.name });
  }, [workspace?.name, reset]);

  const saveMutation = useMutation({
    mutationFn: (data: { name: string }) =>
      updateWorkspaceSettings({
        runtimeBaseURL: apiBaseURL,
        csrfToken,
        body: { name: data.name.trim() },
      }),
    onSuccess: () => {
      showSuccess("Workspace updated");
      queryClient.invalidateQueries({ queryKey });
      queryClient.invalidateQueries({ queryKey: ["ui-session"] });
    },
    onError: (err: Error) => showError(err.message),
  });

  const busy = isSubmitting || saveMutation.isPending;

  const copyID = () => {
    if (!workspace?.id) return;
    navigator.clipboard.writeText(workspace.id);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  if (settingsQuery.isPending) {
    return (
      <div className="flex items-center justify-center py-16">
        <LoaderCircle className="h-5 w-5 animate-spin text-text-muted" />
      </div>
    );
  }

  return (
    <div className="divide-y divide-border">
      {/* Your account */}
      <div className="p-6">
        <SectionHeader title="Your account" description="The account you're signed in with." />
        <div className="mt-4 grid gap-3 max-w-lg">
          <ReadOnlyField label="Email" value={session?.user_email ?? "..."} mono />
          <ReadOnlyField label="Role" value={session?.role ?? "..."} />
          <ProfileNameField apiBaseURL={apiBaseURL} csrfToken={csrfToken} currentName={session?.display_name || ""} />
        </div>
      </div>

      {/* Workspace details */}
      <form onSubmit={handleSubmit((data) => saveMutation.mutate(data))} className="p-6">
        <SectionHeader title="Workspace" description="Your workspace identity and metadata." />
        <div className="mt-4 grid gap-4 max-w-lg">
          <label className="grid gap-1.5">
            <span className="text-xs font-medium text-text-muted">Name</span>
            <input
              type="text"
              placeholder="My workspace"
              className="h-10 rounded-lg border border-border bg-surface px-3 text-sm text-text-primary outline-none ring-slate-400 transition placeholder:text-text-faint focus:ring-2"
              {...register("name")}
            />
          </label>

          <div className="grid gap-1.5">
            <span className="text-xs font-medium text-text-muted">Workspace ID</span>
            <div className="flex items-center gap-2">
              <code className="flex-1 rounded-lg border border-border bg-surface-secondary px-3 py-2.5 font-mono text-xs text-text-secondary">
                {workspace?.id ?? "..."}
              </code>
              <button
                type="button"
                onClick={copyID}
                className="inline-flex h-10 w-10 shrink-0 items-center justify-center rounded-lg border border-border bg-surface text-text-muted transition hover:bg-surface-secondary hover:text-text-primary"
              >
                {copied ? <CheckCircle2 className="h-3.5 w-3.5 text-emerald-600" /> : <Copy className="h-3.5 w-3.5" />}
              </button>
            </div>
          </div>

          <div className="flex items-center gap-6 text-sm text-text-muted">
            {workspace?.status ? (
              <span className="flex items-center gap-1.5">
                <span className={`inline-block h-1.5 w-1.5 rounded-full ${workspace.status === "active" ? "bg-emerald-500" : "bg-amber-500"}`} />
                {workspace.status}
              </span>
            ) : null}
            {workspace?.created_at ? (
              <span>
                Created {new Date(workspace.created_at).toLocaleDateString("en-US", { year: "numeric", month: "short", day: "numeric" })}
              </span>
            ) : null}
          </div>
        </div>

        {/* Save bar — always visible, disabled when clean */}
        <div className="mt-6 flex items-center gap-3 border-t border-border pt-4">
          {isDirty ? <p className="flex-1 text-xs text-amber-600 dark:text-amber-400">Unsaved changes</p> : <span className="flex-1" />}
          <button
            type="submit"
            disabled={!isDirty || busy}
            className="inline-flex h-10 items-center gap-2 rounded-lg border border-slate-900 bg-slate-900 px-5 text-sm font-semibold text-white transition hover:bg-slate-800 disabled:cursor-not-allowed disabled:border-border disabled:bg-surface-secondary disabled:text-text-faint dark:border-white dark:bg-white dark:text-slate-900 dark:hover:bg-slate-100 dark:disabled:border-border dark:disabled:bg-surface-secondary dark:disabled:text-text-faint"
          >
            {busy ? <LoaderCircle className="h-3.5 w-3.5 animate-spin" /> : null}
            Save changes
          </button>
        </div>
      </form>
    </div>
  );
}

function SectionHeader({ title, description }: { title: string; description: string }) {
  return (
    <div>
      <h3 className="text-sm font-semibold text-text-primary">{title}</h3>
      <p className="mt-0.5 text-xs text-text-muted">{description}</p>
    </div>
  );
}

function ReadOnlyField({ label, value, mono }: { label: string; value: string; mono?: boolean }) {
  return (
    <div className="grid gap-1">
      <span className="text-xs font-medium text-text-muted">{label}</span>
      <p className={`text-sm text-text-secondary ${mono ? "font-mono" : ""}`}>{value}</p>
    </div>
  );
}

function ProfileNameField({ apiBaseURL, csrfToken, currentName }: { apiBaseURL: string; csrfToken: string; currentName: string }) {
  const queryClient = useQueryClient();
  const [name, setName] = useState(currentName);
  const [editing, setEditing] = useState(false);

  useEffect(() => { setName(currentName); }, [currentName]);

  const mutation = useMutation({
    mutationFn: () => updateUserProfile({ runtimeBaseURL: apiBaseURL, csrfToken, displayName: name.trim() }),
    onSuccess: () => {
      showSuccess("Display name updated");
      setEditing(false);
      queryClient.invalidateQueries({ queryKey: ["ui-session"] });
    },
    onError: (err: Error) => showError(err.message),
  });

  const dirty = name.trim() !== currentName && name.trim().length > 0;

  if (!editing) {
    return (
      <div className="grid gap-1">
        <span className="text-xs font-medium text-text-muted">Display name</span>
        <div className="flex items-center gap-2">
          <p className="text-sm text-text-secondary">{currentName || "Not set"}</p>
          <button
            type="button"
            onClick={() => setEditing(true)}
            className="text-xs font-medium text-text-faint transition hover:text-text-secondary"
          >
            Edit
          </button>
        </div>
      </div>
    );
  }

  return (
    <div className="grid gap-1">
      <span className="text-xs font-medium text-text-muted">Display name</span>
      <div className="flex items-center gap-2">
        <input
          type="text"
          value={name}
          onChange={(e) => setName(e.target.value)}
          autoFocus
          className="h-9 flex-1 rounded-lg border border-border bg-surface px-3 text-sm text-text-primary outline-none ring-slate-400 transition focus:ring-2"
        />
        <button
          type="button"
          onClick={() => mutation.mutate()}
          disabled={!dirty || mutation.isPending}
          className="inline-flex h-9 items-center gap-1.5 rounded-lg bg-slate-900 px-3 text-xs font-medium text-white transition hover:bg-slate-800 disabled:opacity-50 dark:bg-white dark:text-slate-900"
        >
          {mutation.isPending ? <LoaderCircle className="h-3 w-3 animate-spin" /> : null}
          Save
        </button>
        <button
          type="button"
          onClick={() => { setName(currentName); setEditing(false); }}
          className="text-xs font-medium text-text-faint transition hover:text-text-secondary"
        >
          Cancel
        </button>
      </div>
    </div>
  );
}
