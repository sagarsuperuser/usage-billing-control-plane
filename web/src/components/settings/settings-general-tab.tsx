
import { CheckCircle2, Copy, LoaderCircle } from "lucide-react";
import { useEffect, useState } from "react";
import { useForm } from "react-hook-form";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import { fetchWorkspaceSettings, updateWorkspaceSettings } from "@/lib/api";
import { showError } from "@/lib/toast";

export function SettingsGeneralTab({
  apiBaseURL,
  csrfToken,
}: {
  apiBaseURL: string;
  csrfToken: string;
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

  // eslint-disable-next-line react-hooks/incompatible-library -- reset() from RHF triggers re-render by design
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
    <form onSubmit={handleSubmit((data) => saveMutation.mutate(data))} className="p-6">
      <div className="max-w-lg space-y-6">
        <div>
          <h3 className="text-sm font-semibold text-text-primary">Workspace</h3>
          <p className="mt-0.5 text-xs text-text-muted">General workspace configuration.</p>
        </div>

        <div className="grid gap-4">
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
                className="inline-flex h-10 w-10 items-center justify-center rounded-lg border border-border bg-surface text-text-muted transition hover:bg-surface-secondary hover:text-text-primary"
              >
                {copied ? <CheckCircle2 className="h-3.5 w-3.5 text-emerald-600" /> : <Copy className="h-3.5 w-3.5" />}
              </button>
            </div>
          </div>

          {workspace?.created_at ? (
            <div className="grid gap-1.5">
              <span className="text-xs font-medium text-text-muted">Created</span>
              <p className="text-sm text-text-secondary">
                {new Date(workspace.created_at).toLocaleDateString("en-US", {
                  year: "numeric",
                  month: "long",
                  day: "numeric",
                })}
              </p>
            </div>
          ) : null}
        </div>

        {saveMutation.isSuccess ? (
          <p className="flex items-center gap-1.5 text-xs text-emerald-600">
            <CheckCircle2 className="h-3.5 w-3.5" /> Saved
          </p>
        ) : null}

        <div className="flex gap-2 border-t border-border pt-4">
          <button
            type="submit"
            disabled={!isDirty || busy}
            className="inline-flex h-9 items-center gap-2 rounded-lg bg-slate-900 px-4 text-sm font-medium text-white transition hover:bg-slate-800 disabled:cursor-not-allowed disabled:opacity-50 dark:bg-white dark:text-slate-900 dark:hover:bg-slate-100"
          >
            {busy ? <LoaderCircle className="h-3.5 w-3.5 animate-spin" /> : null}
            Save changes
          </button>
        </div>
      </div>
    </form>
  );
}
