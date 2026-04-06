
import { Button } from "@/components/ui/button";
import { useMemo, useState } from "react";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import {
  Copy,
  KeyRound,
  LoaderCircle,
  Plus,
  RefreshCw,
  Search,
  ServerCog,
  ShieldOff,
  X,
} from "lucide-react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import { ConfirmDialog } from "@/components/ui/confirm-dialog";
import { StatusChip } from "@/components/ui/status-chip";
import { MiniPagination } from "@/components/ui/mini-pagination";

import {
  createTenantWorkspaceServiceAccount,
  fetchTenantWorkspaceServiceAccounts,
  issueTenantWorkspaceServiceAccountCredential,
  revokeTenantWorkspaceServiceAccountCredential,
  rotateTenantWorkspaceServiceAccountCredential,
  updateTenantWorkspaceServiceAccountStatus,
} from "@/lib/api";
import { formatRelativeTimestamp } from "@/lib/format";
import { showError, showSuccess } from "@/lib/toast";

/* ------------------------------------------------------------------ */
/*  Types                                                              */
/* ------------------------------------------------------------------ */

export interface WorkspaceServiceAccountsTabProps {
  apiBaseURL: string;
  csrfToken: string;
  isAdmin: boolean;
  session: { tenant_id?: string; subject_id?: string } | null;
}

/* ------------------------------------------------------------------ */
/*  Component                                                          */
/* ------------------------------------------------------------------ */

export function WorkspaceServiceAccountsTab({ apiBaseURL, csrfToken, session }: WorkspaceServiceAccountsTabProps) {
  const queryClient = useQueryClient();

  const { register: registerSA, handleSubmit: handleSASubmit, reset: resetSA } = useForm({
    resolver: zodResolver(z.object({
      name: z.string().min(1),
      description: z.string(),
      role: z.enum(["reader", "writer", "admin"]),
      purpose: z.string().min(1),
      environment: z.string().min(1),
    })),
    defaultValues: { name: "", description: "", role: "writer" as const, purpose: "", environment: "prod" },
  });

  const [latestCredentialSecret, setLatestCredentialSecret] = useState<{ label: string; secret: string } | null>(null);
  const [selectedServiceAccountID, setSelectedServiceAccountID] = useState("");
  const [serviceAccountPage, setServiceAccountPage] = useState(1);
  const [credentialPage, setCredentialPage] = useState(1);
  const [showNewSAModal, setShowNewSAModal] = useState(false);
  const [search, setSearch] = useState("");

  /* --- Queries ---------------------------------------------------- */

  const serviceAccountQueryKey = ["tenant-workspace-service-accounts", apiBaseURL, session?.tenant_id];

  const serviceAccountsQuery = useQuery({
    queryKey: serviceAccountQueryKey,
    queryFn: () => fetchTenantWorkspaceServiceAccounts({ runtimeBaseURL: apiBaseURL }),
    enabled: Boolean(session),
  });

  /* --- Mutations -------------------------------------------------- */

  const createServiceAccountMutation = useMutation({
    mutationFn: (data: { name: string; description: string; role: "reader" | "writer" | "admin"; purpose: string; environment: string }) =>
      createTenantWorkspaceServiceAccount({
        runtimeBaseURL: apiBaseURL,
        csrfToken,
        name: data.name,
        description: data.description,
        role: data.role,
        purpose: data.purpose,
        environment: data.environment,
        issueInitialCredential: true,
      }),
    onSuccess: async (payload) => {
      resetSA();
      setShowNewSAModal(false);
      setSelectedServiceAccountID(payload.service_account.id);
      if (payload.secret) {
        setLatestCredentialSecret({ label: payload.service_account.name, secret: payload.secret });
      }
      showSuccess("Service account created", "An initial API key has been issued.");
      await queryClient.invalidateQueries({ queryKey: serviceAccountQueryKey });
    },
    onError: (err: Error) => showError(err.message),
  });
  const issueCredentialMutation = useMutation({
    mutationFn: (serviceAccountID: string) =>
      issueTenantWorkspaceServiceAccountCredential({
        runtimeBaseURL: apiBaseURL,
        csrfToken,
        serviceAccountID,
      }),
    onSuccess: async (payload) => {
      setLatestCredentialSecret({ label: payload.credential.name, secret: payload.secret });
      showSuccess("Key issued", "Copy it now — it won't be shown again.");
      await queryClient.invalidateQueries({ queryKey: serviceAccountQueryKey });
    },
    onError: (err: Error) => showError(err.message),
  });
  const rotateCredentialMutation = useMutation({
    mutationFn: (input: { serviceAccountID: string; credentialID: string }) =>
      rotateTenantWorkspaceServiceAccountCredential({
        runtimeBaseURL: apiBaseURL,
        csrfToken,
        serviceAccountID: input.serviceAccountID,
        credentialID: input.credentialID,
      }),
    onSuccess: async (payload) => {
      setLatestCredentialSecret({ label: payload.credential.name, secret: payload.secret });
      showSuccess("Key rotated", "The old key has been revoked. Copy the new one now.");
      await queryClient.invalidateQueries({ queryKey: serviceAccountQueryKey });
    },
    onError: (err: Error) => showError(err.message),
  });
  const revokeCredentialMutation = useMutation({
    mutationFn: (input: { serviceAccountID: string; credentialID: string }) =>
      revokeTenantWorkspaceServiceAccountCredential({
        runtimeBaseURL: apiBaseURL,
        csrfToken,
        serviceAccountID: input.serviceAccountID,
        credentialID: input.credentialID,
      }),
    onSuccess: async () => {
      showSuccess("Key revoked", "This key will no longer authenticate.");
      await queryClient.invalidateQueries({ queryKey: serviceAccountQueryKey });
    },
    onError: (err: Error) => showError(err.message),
  });
  const updateServiceAccountStatusMutation = useMutation({
    mutationFn: (input: { serviceAccountID: string; status: "active" | "disabled" }) =>
      updateTenantWorkspaceServiceAccountStatus({
        runtimeBaseURL: apiBaseURL,
        csrfToken,
        serviceAccountID: input.serviceAccountID,
        status: input.status,
      }),
    // Optimistic update: toggle status instantly, rollback on error
    onMutate: async (input) => {
      await queryClient.cancelQueries({ queryKey: serviceAccountQueryKey });
      const previous = queryClient.getQueryData(serviceAccountQueryKey);
      queryClient.setQueryData(serviceAccountQueryKey, (old: typeof serviceAccounts) =>
        old?.map((sa: typeof serviceAccounts[number]) => sa.id === input.serviceAccountID ? { ...sa, status: input.status } : sa)
      );
      return { previous };
    },
    onSuccess: (_payload, input) => {
      showSuccess(input.status === "active" ? "Service account enabled" : "Service account disabled");
    },
    onError: (err: Error, _input, context) => {
      if (context?.previous) queryClient.setQueryData(serviceAccountQueryKey, context.previous);
      showError(err.message);
    },
    onSettled: () => {
      queryClient.invalidateQueries({ queryKey: serviceAccountQueryKey });
    },
  });

  /* --- Derived ---------------------------------------------------- */

  const serviceAccounts = serviceAccountsQuery.data ?? [];
  const filteredServiceAccounts = useMemo(() => {
    const term = search.trim().toLowerCase();
    if (!term) return serviceAccounts;
    return serviceAccounts.filter(sa =>
      sa.name.toLowerCase().includes(term) ||
      (sa.description || "").toLowerCase().includes(term)
    );
  }, [serviceAccounts, search]);
  const selectedServiceAccountIDValue = selectedServiceAccountID || serviceAccounts[0]?.id || "";
  const selectedServiceAccount =
    serviceAccounts.find((item) => item.id === selectedServiceAccountIDValue) ?? serviceAccounts[0] ?? null;
  const selectedServiceAccountCredentials = selectedServiceAccount?.credentials ?? [];

  const pagedServiceAccounts = paginateItems(filteredServiceAccounts, serviceAccountPage, 8);
  const pagedCredentials = paginateItems(selectedServiceAccountCredentials, credentialPage, 5);

  /* --- Render ----------------------------------------------------- */

  return (
    <>
      {/* New SA modal */}
      {showNewSAModal && (
        <div
          className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4"
          onClick={(e) => { if (e.target === e.currentTarget) setShowNewSAModal(false); }}
        >
          <div className="w-full max-w-lg rounded-xl bg-surface shadow-2xl ring-1 ring-black/10">
            <div className="flex items-center justify-between border-b border-border px-6 py-4">
              <p className="font-semibold text-text-primary">New service account</p>
              <button type="button" onClick={() => setShowNewSAModal(false)} className="inline-flex h-7 w-7 items-center justify-center rounded-lg text-text-faint transition hover:bg-surface-tertiary hover:text-text-secondary">
                <X className="h-4 w-4" />
              </button>
            </div>
            <div className="p-6">
              <div className="grid gap-3">
                <div className="grid grid-cols-2 gap-3">
                  <div>
                    <label className="mb-1.5 block text-xs font-medium text-text-secondary">Name</label>
                    <input {...registerSA("name")} type="text" placeholder="e.g. erp-sync" className="h-9 w-full rounded-lg border border-border bg-surface px-3 text-sm text-text-primary outline-none ring-slate-400 transition focus:ring-2" />
                  </div>
                  <div>
                    <label className="mb-1.5 block text-xs font-medium text-text-secondary">Role</label>
                    <select {...registerSA("role")} aria-label="Service account role" className="h-9 w-full rounded-lg border border-border bg-surface px-3 text-sm text-text-primary outline-none ring-slate-400 transition focus:ring-2">
                      <option value="admin">Admin</option>
                      <option value="writer">Writer</option>
                      <option value="reader">Reader</option>
                    </select>
                  </div>
                </div>
                <div className="grid grid-cols-2 gap-3">
                  <div>
                    <label className="mb-1.5 block text-xs font-medium text-text-secondary">Purpose</label>
                    <input {...registerSA("purpose")} type="text" placeholder="e.g. erp sync" className="h-9 w-full rounded-lg border border-border bg-surface px-3 text-sm text-text-primary outline-none ring-slate-400 transition focus:ring-2" />
                  </div>
                  <div>
                    <label className="mb-1.5 block text-xs font-medium text-text-secondary">Environment</label>
                    <input {...registerSA("environment")} type="text" placeholder="prod" className="h-9 w-full rounded-lg border border-border bg-surface px-3 text-sm text-text-primary outline-none ring-slate-400 transition focus:ring-2" />
                  </div>
                </div>
                <div>
                  <label className="mb-1.5 block text-xs font-medium text-text-secondary">Description <span className="font-normal text-text-faint">(optional)</span></label>
                  <textarea {...registerSA("description")} placeholder="What this account is used for" rows={2} className="w-full rounded-lg border border-border bg-surface px-3 py-2 text-sm text-text-primary outline-none ring-slate-400 transition focus:ring-2" />
                </div>
              </div>
              <div className="mt-4 flex justify-end gap-2 border-t border-border-light pt-4">
                <button type="button" onClick={() => setShowNewSAModal(false)} className="inline-flex h-9 items-center rounded-lg border border-border px-4 text-sm text-text-secondary transition hover:bg-surface-tertiary">
                  Cancel
                </button>
                <button
                  type="button"
                  onClick={handleSASubmit((data) => createServiceAccountMutation.mutate(data))}
                  disabled={!csrfToken || createServiceAccountMutation.isPending}
                  className="inline-flex h-9 items-center gap-2 rounded-lg bg-blue-600 px-4 text-sm font-medium text-white transition hover:bg-blue-700 disabled:opacity-50"
                >
                  {createServiceAccountMutation.isPending ? <LoaderCircle className="h-4 w-4 animate-spin" /> : <ServerCog className="h-4 w-4" />}
                  Create
                </button>
              </div>
            </div>
          </div>
        </div>
      )}

      {/* Main layout: left list + right detail */}
      <div className="flex divide-x divide-border">
        {/* Left: service account list */}
        <div className={`shrink-0 ${selectedServiceAccount ? "w-[280px]" : "w-full max-w-sm"}`}>
          <div className="flex items-center gap-2 border-b border-border px-4 py-2.5">
            <div className="relative flex-1">
              <Search className="pointer-events-none absolute left-2.5 top-1/2 h-3.5 w-3.5 -translate-y-1/2 text-text-faint" />
              <input
                type="text"
                value={search}
                onChange={(e) => { setSearch(e.target.value); setServiceAccountPage(1); }}
                aria-label="Search" placeholder="Search..."
                className="h-7 w-full rounded border border-border bg-surface pl-8 pr-3 text-xs text-text-primary outline-none ring-slate-400 transition placeholder:text-text-faint focus:ring-1"
              />
            </div>
            <button
              type="button"
              onClick={() => { resetSA(); setLatestCredentialSecret(null); setShowNewSAModal(true); }}
              disabled={!csrfToken}
              className="inline-flex h-7 items-center gap-1 rounded bg-blue-600 px-2.5 text-xs font-medium text-white transition hover:bg-blue-700 disabled:opacity-50"
            >
              <Plus className="h-3 w-3" />
              New
            </button>
          </div>
          <div className="flex items-center justify-between border-b border-border-light px-4 py-1.5">
            <p className="text-[11px] font-medium text-text-faint">{filteredServiceAccounts.length} account{filteredServiceAccounts.length === 1 ? "" : "s"}</p>
            <MiniPagination page={pagedServiceAccounts.page} totalPages={pagedServiceAccounts.totalPages} onPageChange={setServiceAccountPage} label="Service accounts" />
          </div>
          {pagedServiceAccounts.items.length > 0 ? (
            <div className="divide-y divide-border-light">
              {pagedServiceAccounts.items.map((account) => {
                const selected = selectedServiceAccountIDValue === account.id;
                return (
                  <div
                    key={account.id}
                    data-testid={`inspect-service-account-${account.id}`}
                    onClick={() => setSelectedServiceAccountID(selected ? "" : account.id)}
                    className={`cursor-pointer px-4 py-2.5 transition ${selected ? "bg-sky-50 border-l-2 border-l-sky-500" : "hover:bg-surface-secondary border-l-2 border-l-transparent"}`}
                  >
                    <div className="flex items-center justify-between gap-2">
                      <p className="truncate text-sm font-medium text-text-primary">{account.name}</p>
                      {account.status === "active" ? (
                        <span className="h-1.5 w-1.5 shrink-0 rounded-full bg-emerald-500" title="Active" />
                      ) : (
                        <span className="h-1.5 w-1.5 shrink-0 rounded-full bg-stone-300" title="Disabled" />
                      )}
                    </div>
                    <p className="mt-0.5 text-[11px] text-text-muted">
                      {formatRole(account.role)}
                      {account.environment ? ` \u00b7 ${account.environment}` : ""}
                      {` \u00b7 ${account.active_credential_count} key${account.active_credential_count === 1 ? "" : "s"}`}
                    </p>
                  </div>
                );
              })}
            </div>
          ) : (
            <div className="flex flex-col items-center gap-2 px-4 py-10 text-center">
              <ServerCog className="h-6 w-6 text-slate-300" />
              <p className="text-xs text-text-muted">No service accounts yet</p>
            </div>
          )}
        </div>

        {/* Right: detail panel */}
        {selectedServiceAccount && (
          <div data-testid="service-account-detail" className="min-w-0 flex-1">
            {/* Header */}
            <div className="flex items-center justify-between border-b border-border px-5 py-3">
              <div className="min-w-0">
                <div className="flex items-center gap-2">
                  <p className="truncate text-sm font-semibold text-text-primary">{selectedServiceAccount.name}</p>
                  <StatusChip tone={selectedServiceAccount.status === "active" ? "success" : "neutral"}>
                    {selectedServiceAccount.status === "active" ? "Active" : "Disabled"}
                  </StatusChip>
                </div>
                {selectedServiceAccount.description ? (
                  <p className="mt-0.5 truncate text-xs text-text-muted">{selectedServiceAccount.description}</p>
                ) : null}
              </div>
              <button type="button" onClick={() => setSelectedServiceAccountID("")} className="inline-flex h-6 w-6 items-center justify-center rounded text-text-faint transition hover:bg-surface-tertiary hover:text-text-secondary">
                <X className="h-3.5 w-3.5" />
              </button>
            </div>

            {/* Metadata row */}
            <div className="flex flex-wrap gap-x-6 gap-y-1 border-b border-border-light px-5 py-2.5 text-xs">
              <span className="text-text-muted">Role <span className="font-medium text-text-secondary">{formatRole(selectedServiceAccount.role)}</span></span>
              {selectedServiceAccount.environment ? <span className="text-text-muted">Env <span className="font-medium text-text-secondary">{selectedServiceAccount.environment}</span></span> : null}
              {selectedServiceAccount.purpose ? <span className="text-text-muted">Purpose <span className="font-medium text-text-secondary">{selectedServiceAccount.purpose}</span></span> : null}
              <span className="text-text-muted">Created <span className="font-medium text-text-secondary">{formatRelativeTimestamp(selectedServiceAccount.created_at)}</span></span>
            </div>

            {/* Actions */}
            <div className="flex items-center gap-2 border-b border-border px-5 py-2.5">
              <button
                type="button"
                onClick={() => issueCredentialMutation.mutate(selectedServiceAccount.id)}
                disabled={!csrfToken || issueCredentialMutation.isPending || selectedServiceAccount.status !== "active"}
                className="inline-flex h-7 items-center gap-1.5 rounded bg-blue-600 px-2.5 text-xs font-medium text-white transition hover:bg-blue-700 disabled:opacity-50"
              >
                {issueCredentialMutation.isPending ? <LoaderCircle className="h-3 w-3 animate-spin" /> : <KeyRound className="h-3 w-3" />}
                Issue key
              </button>
              <ConfirmDialog
                title={selectedServiceAccount.status === "active" ? "Disable this service account?" : "Enable this service account?"}
                description={selectedServiceAccount.status === "active" ? "All API keys under this account will stop authenticating immediately." : "This account and its active keys will resume authenticating."}
                confirmLabel={selectedServiceAccount.status === "active" ? "Disable" : "Enable"}
                tone={selectedServiceAccount.status === "active" ? "danger" : undefined}
                onConfirm={async () => { await updateServiceAccountStatusMutation.mutateAsync({ serviceAccountID: selectedServiceAccount.id, status: selectedServiceAccount.status === "active" ? "disabled" : "active" }); }}
              >
                {(open) => (
                  <button
                    type="button"
                    onClick={open}
                    disabled={!csrfToken || updateServiceAccountStatusMutation.isPending}
                    className="inline-flex h-7 items-center gap-1.5 rounded border border-border px-2.5 text-xs font-medium text-text-muted transition hover:bg-surface-tertiary disabled:opacity-50"
                  >
                    <ShieldOff className="h-3 w-3" />
                    {selectedServiceAccount.status === "active" ? "Disable" : "Enable"}
                  </button>
                )}
              </ConfirmDialog>
            </div>

            {/* Secret banner */}
            {latestCredentialSecret && (
              <div className="border-b border-amber-200 bg-amber-50 px-5 py-3">
                <div className="flex items-start justify-between gap-2">
                  <div className="min-w-0 flex-1">
                    <p className="text-xs font-semibold text-amber-800">Copy this key now &mdash; it won&apos;t be shown again</p>
                    <div className="mt-1.5 flex items-center gap-2">
                      <code className="block min-w-0 flex-1 truncate rounded border border-amber-100 bg-surface px-2 py-1 font-mono text-xs text-text-secondary">{latestCredentialSecret.secret}</code>
                      <button type="button" onClick={() => { void navigator.clipboard.writeText(latestCredentialSecret.secret); showSuccess("Copied to clipboard"); }} className="inline-flex h-7 shrink-0 items-center gap-1 rounded border border-amber-200 bg-surface px-2 text-xs text-amber-800 hover:bg-amber-100">
                        <Copy className="h-3 w-3" />
                        Copy
                      </button>
                    </div>
                  </div>
                  <button type="button" onClick={() => setLatestCredentialSecret(null)} className="inline-flex h-5 w-5 shrink-0 items-center justify-center rounded text-amber-400 hover:text-amber-600">
                    <X className="h-3.5 w-3.5" />
                  </button>
                </div>
              </div>
            )}

            {/* Credentials list */}
            <div>
              <div className="flex items-center justify-between px-5 py-2.5">
                <p className="text-xs font-semibold text-text-secondary">API keys ({selectedServiceAccountCredentials.length})</p>
                <MiniPagination page={pagedCredentials.page} totalPages={pagedCredentials.totalPages} onPageChange={setCredentialPage} label="Credentials" />
              </div>
              {pagedCredentials.items.length > 0 ? (
                <div className="divide-y divide-border-light">
                  {pagedCredentials.items.map((credential) => {
                    const isRevoked = Boolean(credential.revoked_at);
                    const prefix = credential.key_prefix ? credential.key_prefix.slice(0, 8) : "--------";
                    return (
                      <div key={credential.id} className="flex items-center gap-3 px-5 py-2.5">
                        <div className="min-w-0 flex-1">
                          <div className="flex items-center gap-2">
                            <code className="text-xs font-medium text-text-secondary">sk_{prefix}...</code>
                            <StatusChip tone={isRevoked ? "danger" : "success"}>{isRevoked ? "Revoked" : "Active"}</StatusChip>
                          </div>
                          <p className="mt-0.5 text-[11px] text-text-faint">
                            {isRevoked
                              ? `Revoked ${formatRelativeTimestamp(credential.revoked_at)}`
                              : credential.last_used_at
                                ? `Used ${formatRelativeTimestamp(credential.last_used_at)}`
                                : `Created ${formatRelativeTimestamp(credential.created_at)}`}
                          </p>
                        </div>
                        {!isRevoked && (
                          <div className="flex shrink-0 gap-1">
                            <Button variant="secondary" size="xs" onClick={() => rotateCredentialMutation.mutate({ serviceAccountID: selectedServiceAccount.id, credentialID: credential.id })} disabled={!csrfToken} loading={rotateCredentialMutation.isPending}>
                              {!rotateCredentialMutation.isPending ? <RefreshCw className="h-2.5 w-2.5" /> : null}
                              Roll
                            </Button>
                            <ConfirmDialog
                              title="Revoke this key?"
                              description="This key will immediately stop working. Any integrations using it will break. This cannot be undone."
                              confirmLabel="Revoke key"
                              onConfirm={async () => { await revokeCredentialMutation.mutateAsync({ serviceAccountID: selectedServiceAccount.id, credentialID: credential.id }); }}
                            >
                              {(open) => (
                                <Button variant="danger" size="xs" onClick={open} disabled={!csrfToken} loading={revokeCredentialMutation.isPending}>Revoke</Button>
                              )}
                            </ConfirmDialog>
                          </div>
                        )}
                      </div>
                    );
                  })}
                </div>
              ) : (
                <p className="px-5 py-4 text-xs text-text-faint">No keys issued yet.</p>
              )}
            </div>
          </div>
        )}
      </div>
    </>
  );
}

/* ------------------------------------------------------------------ */
/*  Helpers                                                            */
/* ------------------------------------------------------------------ */

function paginateItems<T>(items: T[], requestedPage: number, pageSize: number): { items: T[]; page: number; totalPages: number } {
  const totalPages = Math.max(1, Math.ceil(items.length / pageSize));
  const page = Math.min(Math.max(requestedPage, 1), totalPages);
  const start = (page - 1) * pageSize;
  return { items: items.slice(start, start + pageSize), page, totalPages };
}

function formatRole(role: string): string {
  return role === "admin" ? "Admin" : role === "writer" ? "Writer" : "Reader";
}
