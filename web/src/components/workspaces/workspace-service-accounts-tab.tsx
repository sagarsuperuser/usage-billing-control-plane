"use client";

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
      await queryClient.invalidateQueries({ queryKey: serviceAccountQueryKey });
    },
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
      await queryClient.invalidateQueries({ queryKey: serviceAccountQueryKey });
    },
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
      await queryClient.invalidateQueries({ queryKey: serviceAccountQueryKey });
    },
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
      await queryClient.invalidateQueries({ queryKey: serviceAccountQueryKey });
    },
  });
  const updateServiceAccountStatusMutation = useMutation({
    mutationFn: (input: { serviceAccountID: string; status: "active" | "disabled" }) =>
      updateTenantWorkspaceServiceAccountStatus({
        runtimeBaseURL: apiBaseURL,
        csrfToken,
        serviceAccountID: input.serviceAccountID,
        status: input.status,
      }),
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: serviceAccountQueryKey });
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
          <div className="w-full max-w-lg rounded-xl bg-white shadow-2xl ring-1 ring-black/10">
            <div className="flex items-center justify-between border-b border-stone-200 px-6 py-4">
              <p className="font-semibold text-slate-900">New service account</p>
              <button type="button" onClick={() => setShowNewSAModal(false)} className="inline-flex h-7 w-7 items-center justify-center rounded-lg text-slate-400 transition hover:bg-stone-100 hover:text-slate-700">
                <X className="h-4 w-4" />
              </button>
            </div>
            <div className="p-6">
              <div className="grid gap-3">
                <div className="grid grid-cols-2 gap-3">
                  <div>
                    <label className="mb-1.5 block text-xs font-medium text-slate-700">Name</label>
                    <input {...registerSA("name")} type="text" placeholder="e.g. erp-sync" className="h-9 w-full rounded-lg border border-stone-200 bg-white px-3 text-sm text-slate-900 outline-none ring-slate-400 transition focus:ring-2" />
                  </div>
                  <div>
                    <label className="mb-1.5 block text-xs font-medium text-slate-700">Role</label>
                    <select {...registerSA("role")} aria-label="Service account role" className="h-9 w-full rounded-lg border border-stone-200 bg-white px-3 text-sm text-slate-900 outline-none ring-slate-400 transition focus:ring-2">
                      <option value="admin">Admin</option>
                      <option value="writer">Writer</option>
                      <option value="reader">Reader</option>
                    </select>
                  </div>
                </div>
                <div className="grid grid-cols-2 gap-3">
                  <div>
                    <label className="mb-1.5 block text-xs font-medium text-slate-700">Purpose</label>
                    <input {...registerSA("purpose")} type="text" placeholder="e.g. erp sync" className="h-9 w-full rounded-lg border border-stone-200 bg-white px-3 text-sm text-slate-900 outline-none ring-slate-400 transition focus:ring-2" />
                  </div>
                  <div>
                    <label className="mb-1.5 block text-xs font-medium text-slate-700">Environment</label>
                    <input {...registerSA("environment")} type="text" placeholder="prod" className="h-9 w-full rounded-lg border border-stone-200 bg-white px-3 text-sm text-slate-900 outline-none ring-slate-400 transition focus:ring-2" />
                  </div>
                </div>
                <div>
                  <label className="mb-1.5 block text-xs font-medium text-slate-700">Description <span className="font-normal text-slate-400">(optional)</span></label>
                  <textarea {...registerSA("description")} placeholder="What this account is used for" rows={2} className="w-full rounded-lg border border-stone-200 bg-white px-3 py-2 text-sm text-slate-900 outline-none ring-slate-400 transition focus:ring-2" />
                </div>
              </div>
              <div className="mt-4 flex justify-end gap-2 border-t border-stone-100 pt-4">
                <button type="button" onClick={() => setShowNewSAModal(false)} className="inline-flex h-9 items-center rounded-lg border border-stone-200 px-4 text-sm text-slate-700 transition hover:bg-stone-100">
                  Cancel
                </button>
                <button
                  type="button"
                  onClick={handleSASubmit((data) => createServiceAccountMutation.mutate(data))}
                  disabled={!csrfToken || createServiceAccountMutation.isPending}
                  className="inline-flex h-9 items-center gap-2 rounded-lg bg-slate-900 px-4 text-sm font-medium text-white transition hover:bg-slate-800 disabled:opacity-50"
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
      <div className="flex divide-x divide-stone-200">
        {/* Left: service account list */}
        <div className={`shrink-0 ${selectedServiceAccount ? "w-[280px]" : "w-full max-w-sm"}`}>
          <div className="flex items-center gap-2 border-b border-stone-200 px-4 py-2.5">
            <div className="relative flex-1">
              <Search className="pointer-events-none absolute left-2.5 top-1/2 h-3.5 w-3.5 -translate-y-1/2 text-slate-400" />
              <input
                type="text"
                value={search}
                onChange={(e) => { setSearch(e.target.value); setServiceAccountPage(1); }}
                placeholder="Search..."
                className="h-7 w-full rounded border border-stone-200 bg-white pl-8 pr-3 text-xs text-slate-900 outline-none ring-slate-400 transition placeholder:text-slate-400 focus:ring-1"
              />
            </div>
            <button
              type="button"
              onClick={() => { resetSA(); setLatestCredentialSecret(null); setShowNewSAModal(true); }}
              disabled={!csrfToken}
              className="inline-flex h-7 items-center gap-1 rounded bg-slate-900 px-2.5 text-xs font-medium text-white transition hover:bg-slate-800 disabled:opacity-50"
            >
              <Plus className="h-3 w-3" />
              New
            </button>
          </div>
          <div className="flex items-center justify-between border-b border-stone-100 px-4 py-1.5">
            <p className="text-[11px] font-medium text-slate-400">{filteredServiceAccounts.length} account{filteredServiceAccounts.length === 1 ? "" : "s"}</p>
            <MiniPagination page={pagedServiceAccounts.page} totalPages={pagedServiceAccounts.totalPages} onPageChange={setServiceAccountPage} label="Service accounts" />
          </div>
          {pagedServiceAccounts.items.length > 0 ? (
            <div className="divide-y divide-stone-100">
              {pagedServiceAccounts.items.map((account) => {
                const selected = selectedServiceAccountIDValue === account.id;
                return (
                  <div
                    key={account.id}
                    data-testid={`inspect-service-account-${account.id}`}
                    onClick={() => setSelectedServiceAccountID(selected ? "" : account.id)}
                    className={`cursor-pointer px-4 py-2.5 transition ${selected ? "bg-sky-50 border-l-2 border-l-sky-500" : "hover:bg-stone-50 border-l-2 border-l-transparent"}`}
                  >
                    <div className="flex items-center justify-between gap-2">
                      <p className="truncate text-sm font-medium text-slate-900">{account.name}</p>
                      {account.status === "active" ? (
                        <span className="h-1.5 w-1.5 shrink-0 rounded-full bg-emerald-500" title="Active" />
                      ) : (
                        <span className="h-1.5 w-1.5 shrink-0 rounded-full bg-stone-300" title="Disabled" />
                      )}
                    </div>
                    <p className="mt-0.5 text-[11px] text-slate-500">
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
              <p className="text-xs text-slate-500">No service accounts yet</p>
            </div>
          )}
        </div>

        {/* Right: detail panel */}
        {selectedServiceAccount && (
          <div data-testid="service-account-detail" className="min-w-0 flex-1">
            {/* Header */}
            <div className="flex items-center justify-between border-b border-stone-200 px-5 py-3">
              <div className="min-w-0">
                <div className="flex items-center gap-2">
                  <p className="truncate text-sm font-semibold text-slate-900">{selectedServiceAccount.name}</p>
                  <StatusChip tone={selectedServiceAccount.status === "active" ? "success" : "neutral"}>
                    {selectedServiceAccount.status === "active" ? "Active" : "Disabled"}
                  </StatusChip>
                </div>
                {selectedServiceAccount.description ? (
                  <p className="mt-0.5 truncate text-xs text-slate-500">{selectedServiceAccount.description}</p>
                ) : null}
              </div>
              <button type="button" onClick={() => setSelectedServiceAccountID("")} className="inline-flex h-6 w-6 items-center justify-center rounded text-slate-400 transition hover:bg-stone-100 hover:text-slate-700">
                <X className="h-3.5 w-3.5" />
              </button>
            </div>

            {/* Metadata row */}
            <div className="flex flex-wrap gap-x-6 gap-y-1 border-b border-stone-100 px-5 py-2.5 text-xs">
              <span className="text-slate-500">Role <span className="font-medium text-slate-700">{formatRole(selectedServiceAccount.role)}</span></span>
              {selectedServiceAccount.environment ? <span className="text-slate-500">Env <span className="font-medium text-slate-700">{selectedServiceAccount.environment}</span></span> : null}
              {selectedServiceAccount.purpose ? <span className="text-slate-500">Purpose <span className="font-medium text-slate-700">{selectedServiceAccount.purpose}</span></span> : null}
              <span className="text-slate-500">Created <span className="font-medium text-slate-700">{formatRelativeTimestamp(selectedServiceAccount.created_at)}</span></span>
            </div>

            {/* Actions */}
            <div className="flex items-center gap-2 border-b border-stone-200 px-5 py-2.5">
              <button
                type="button"
                onClick={() => issueCredentialMutation.mutate(selectedServiceAccount.id)}
                disabled={!csrfToken || issueCredentialMutation.isPending || selectedServiceAccount.status !== "active"}
                className="inline-flex h-7 items-center gap-1.5 rounded bg-slate-900 px-2.5 text-xs font-medium text-white transition hover:bg-slate-800 disabled:opacity-50"
              >
                {issueCredentialMutation.isPending ? <LoaderCircle className="h-3 w-3 animate-spin" /> : <KeyRound className="h-3 w-3" />}
                Issue key
              </button>
              <button
                type="button"
                onClick={() => updateServiceAccountStatusMutation.mutate({ serviceAccountID: selectedServiceAccount.id, status: selectedServiceAccount.status === "active" ? "disabled" : "active" })}
                disabled={!csrfToken || updateServiceAccountStatusMutation.isPending}
                className="inline-flex h-7 items-center gap-1.5 rounded border border-stone-200 px-2.5 text-xs font-medium text-slate-600 transition hover:bg-stone-100 disabled:opacity-50"
              >
                <ShieldOff className="h-3 w-3" />
                {selectedServiceAccount.status === "active" ? "Disable" : "Enable"}
              </button>
            </div>

            {/* Secret banner */}
            {latestCredentialSecret && (
              <div className="border-b border-amber-200 bg-amber-50 px-5 py-3">
                <div className="flex items-start justify-between gap-2">
                  <div className="min-w-0 flex-1">
                    <p className="text-xs font-semibold text-amber-800">Copy this key now &mdash; it won&apos;t be shown again</p>
                    <div className="mt-1.5 flex items-center gap-2">
                      <code className="block min-w-0 flex-1 truncate rounded border border-amber-100 bg-white px-2 py-1 font-mono text-xs text-slate-800">{latestCredentialSecret.secret}</code>
                      <button type="button" onClick={() => { void navigator.clipboard.writeText(latestCredentialSecret.secret); }} className="inline-flex h-7 shrink-0 items-center gap-1 rounded border border-amber-200 bg-white px-2 text-xs text-amber-800 hover:bg-amber-100">
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
                <p className="text-xs font-semibold text-slate-700">API keys ({selectedServiceAccountCredentials.length})</p>
                <MiniPagination page={pagedCredentials.page} totalPages={pagedCredentials.totalPages} onPageChange={setCredentialPage} label="Credentials" />
              </div>
              {pagedCredentials.items.length > 0 ? (
                <div className="divide-y divide-stone-100">
                  {pagedCredentials.items.map((credential) => {
                    const isRevoked = Boolean(credential.revoked_at);
                    const prefix = credential.key_prefix ? credential.key_prefix.slice(0, 8) : "--------";
                    return (
                      <div key={credential.id} className="flex items-center gap-3 px-5 py-2.5">
                        <div className="min-w-0 flex-1">
                          <div className="flex items-center gap-2">
                            <code className="text-xs font-medium text-slate-800">sk_{prefix}...</code>
                            <StatusChip tone={isRevoked ? "danger" : "success"}>{isRevoked ? "Revoked" : "Active"}</StatusChip>
                          </div>
                          <p className="mt-0.5 text-[11px] text-slate-400">
                            {isRevoked
                              ? `Revoked ${formatRelativeTimestamp(credential.revoked_at)}`
                              : credential.last_used_at
                                ? `Used ${formatRelativeTimestamp(credential.last_used_at)}`
                                : `Created ${formatRelativeTimestamp(credential.created_at)}`}
                          </p>
                        </div>
                        {!isRevoked && (
                          <div className="flex shrink-0 gap-1">
                            <button type="button" onClick={() => rotateCredentialMutation.mutate({ serviceAccountID: selectedServiceAccount.id, credentialID: credential.id })} disabled={!csrfToken || rotateCredentialMutation.isPending} className="inline-flex h-6 items-center gap-1 rounded border border-stone-200 px-1.5 text-[11px] font-medium text-slate-500 transition hover:bg-stone-100 disabled:opacity-50">
                              <RefreshCw className="h-2.5 w-2.5" />
                              Roll
                            </button>
                            <ConfirmDialog
                              title="Revoke this key?"
                              description="This key will immediately stop working. Any integrations using it will break. This cannot be undone."
                              confirmLabel="Revoke key"
                              onConfirm={async () => { await revokeCredentialMutation.mutateAsync({ serviceAccountID: selectedServiceAccount.id, credentialID: credential.id }); }}
                            >
                              {(open) => (
                                <button type="button" onClick={open} disabled={!csrfToken || revokeCredentialMutation.isPending} className="inline-flex h-6 items-center rounded border border-rose-200 px-1.5 text-[11px] font-medium text-rose-600 transition hover:bg-rose-50 disabled:opacity-50">Revoke</button>
                              )}
                            </ConfirmDialog>
                          </div>
                        )}
                      </div>
                    );
                  })}
                </div>
              ) : (
                <p className="px-5 py-4 text-xs text-slate-400">No keys issued yet.</p>
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
