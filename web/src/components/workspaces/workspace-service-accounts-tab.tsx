"use client";

import { type ReactNode, useMemo, useState } from "react";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import {
  ChevronLeft,
  ChevronRight,
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

import {
  createTenantWorkspaceServiceAccount,
  fetchTenantWorkspaceServiceAccounts,
  issueTenantWorkspaceServiceAccountCredential,
  revokeTenantWorkspaceServiceAccountCredential,
  rotateTenantWorkspaceServiceAccountCredential,
  updateTenantWorkspaceServiceAccountStatus,
} from "@/lib/api";
import { formatExactTimestamp } from "@/lib/format";

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

  const pagedServiceAccounts = paginateItems(filteredServiceAccounts, serviceAccountPage, 5);
  const pagedCredentials = paginateItems(selectedServiceAccountCredentials, credentialPage, 4);

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
              <div>
                <p className="font-semibold text-slate-900">New service account</p>
                <p className="mt-0.5 text-xs text-slate-500">Creates a machine identity and issues its first API credential.</p>
              </div>
              <button
                type="button"
                onClick={() => setShowNewSAModal(false)}
                className="inline-flex h-8 w-8 items-center justify-center rounded-lg border border-stone-200 text-slate-400 transition hover:bg-stone-100 hover:text-slate-700"
              >
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
                  <textarea {...registerSA("description")} placeholder="Short description of what this account does" rows={2} className="w-full rounded-lg border border-stone-200 bg-white px-3 py-2 text-sm text-slate-900 outline-none ring-slate-400 transition focus:ring-2" />
                </div>
              </div>
              <div className="mt-4 flex justify-end gap-2 border-t border-stone-100 pt-4">
                <button
                  type="button"
                  onClick={() => setShowNewSAModal(false)}
                  className="inline-flex h-9 items-center rounded-lg border border-stone-200 px-4 text-sm text-slate-700 transition hover:bg-stone-100"
                >
                  Cancel
                </button>
                <button
                  type="button"
                  onClick={handleSASubmit((data) => createServiceAccountMutation.mutate(data))}
                  disabled={!csrfToken || createServiceAccountMutation.isPending}
                  className="inline-flex h-9 items-center gap-2 rounded-lg border border-slate-900 bg-slate-900 px-4 text-sm font-medium text-white transition hover:bg-slate-800 disabled:cursor-not-allowed disabled:opacity-50"
                >
                  {createServiceAccountMutation.isPending ? <LoaderCircle className="h-4 w-4 animate-spin" /> : <ServerCog className="h-4 w-4" />}
                  Create and issue credential
                </button>
              </div>
            </div>
          </div>
        </div>
      )}

      {/* Main layout: table + optional detail panel (58/42 split) */}
      <div className={`flex ${selectedServiceAccount ? "divide-x divide-stone-200" : ""}`}>
        <div className={`min-w-0 ${selectedServiceAccount ? "w-[58%]" : "w-full"}`}>
          <div className="flex items-center justify-between border-b border-stone-200 px-6 py-4">
            <div>
              <p className="text-sm font-semibold text-slate-900">Service accounts</p>
              <p className="mt-0.5 text-xs text-slate-500">API identities for automation and integrations. Issue or rotate credentials as needed.</p>
            </div>
            <div className="flex items-center gap-2">
              <div className="relative">
                <Search className="pointer-events-none absolute left-2.5 top-1/2 h-3.5 w-3.5 -translate-y-1/2 text-slate-400" />
                <input
                  type="text"
                  value={search}
                  onChange={(e) => { setSearch(e.target.value); setServiceAccountPage(1); }}
                  placeholder="Search accounts..."
                  className="h-8 w-44 rounded-lg border border-stone-200 bg-white pl-8 pr-3 text-xs text-slate-900 outline-none ring-slate-400 transition placeholder:text-slate-400 focus:ring-2"
                />
              </div>
              <PaginationControls page={pagedServiceAccounts.page} totalPages={pagedServiceAccounts.totalPages} onPageChange={setServiceAccountPage} label="Service accounts" />
              <button
                type="button"
                onClick={() => { resetSA(); setLatestCredentialSecret(null); setShowNewSAModal(true); }}
                disabled={!csrfToken}
                className="inline-flex h-8 items-center gap-1.5 rounded-lg border border-slate-900 bg-slate-900 px-3 text-xs font-medium text-white transition hover:bg-slate-800 disabled:cursor-not-allowed disabled:opacity-50"
              >
                <Plus className="h-3.5 w-3.5" />
                New
              </button>
            </div>
          </div>
          {pagedServiceAccounts.items.length > 0 ? (
            <table className="w-full text-sm">
              <thead>
                <tr className="border-b border-stone-100 text-left text-xs font-medium text-slate-400">
                  <th className="px-6 py-2.5 font-semibold">Name</th>
                  <th className="px-4 py-2.5 font-semibold">Role</th>
                  <th className="px-4 py-2.5 font-semibold">Env</th>
                  <th className="px-4 py-2.5 font-semibold">Status</th>
                  <th className="px-4 py-2.5 font-semibold">Credentials</th>
                  <th className="px-4 py-2.5 font-semibold">Last used</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-stone-100">
                {pagedServiceAccounts.items.map((account) => {
                  const selected = selectedServiceAccountIDValue === account.id;
                  return (
                    <tr
                      key={account.id}
                      data-testid={`inspect-service-account-${account.id}`}
                      onClick={() => setSelectedServiceAccountID(selected ? "" : account.id)}
                      className={`cursor-pointer transition ${selected ? "bg-sky-50" : "hover:bg-stone-50"}`}
                    >
                      <td className="px-6 py-3.5">
                        <p className="font-medium text-slate-900">{account.name}</p>
                        <p className="mt-0.5 truncate text-xs text-slate-500">{account.description || "No description"}</p>
                      </td>
                      <td className="px-4 py-3.5 text-slate-600">{formatServiceAccountRole(account.role)}</td>
                      <td className="px-4 py-3.5 text-slate-500">{account.environment || "\u2014"}</td>
                      <td className="px-4 py-3.5">
                        <StatusChip tone={account.status === "active" ? "success" : "neutral"}>{formatServiceAccountStatus(account.status)}</StatusChip>
                      </td>
                      <td className="px-4 py-3.5">
                        {account.active_credential_count === 0 ? (
                          <StatusChip tone="warning">None</StatusChip>
                        ) : (
                          <span className="text-slate-600">{account.active_credential_count} active</span>
                        )}
                      </td>
                      <td className="px-4 py-3.5 text-xs text-slate-500">{describeLastUsed(account)}</td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          ) : (
            <div className="flex flex-col items-center justify-center gap-3 px-6 py-16 text-center">
              <ServerCog className="h-8 w-8 text-slate-300" />
              <div>
                <p className="text-sm font-medium text-slate-700">No service accounts yet</p>
                <p className="mt-1 text-xs text-slate-500">Create one to issue API credentials for automation.</p>
              </div>
              <button
                type="button"
                onClick={() => { resetSA(); setLatestCredentialSecret(null); setShowNewSAModal(true); }}
                disabled={!csrfToken}
                className="inline-flex h-9 items-center gap-2 rounded-lg border border-slate-900 bg-slate-900 px-4 text-sm font-medium text-white hover:bg-slate-800 disabled:opacity-50"
              >
                <Plus className="h-3.5 w-3.5" />
                New service account
              </button>
            </div>
          )}
        </div>

        {/* Detail panel */}
        {selectedServiceAccount && (
          <div data-testid="service-account-detail" className="w-[42%] shrink-0 overflow-y-auto">
            <div className="flex items-start justify-between gap-2 border-b border-stone-200 px-5 py-4">
              <div className="min-w-0">
                <p className="font-semibold text-slate-900">{selectedServiceAccount.name}</p>
                <p className="mt-0.5 text-xs text-slate-500">{selectedServiceAccount.description || "No description"}</p>
                <p className="mt-2 text-xs text-slate-500">
                  <span className="font-medium text-slate-700">{formatServiceAccountRole(selectedServiceAccount.role)}</span>
                  {selectedServiceAccount.environment ? <> · <span className="font-medium text-slate-700">{selectedServiceAccount.environment}</span></> : null}
                  {selectedServiceAccount.purpose ? <> · <span className="text-slate-500">{selectedServiceAccount.purpose}</span></> : null}
                  {" · "}
                  <span className="text-slate-500">{describeServiceAccountActivity(selectedServiceAccount)}</span>
                </p>
              </div>
              <div className="flex shrink-0 items-center gap-1.5">
                <StatusChip tone={selectedServiceAccount.status === "active" ? "success" : "neutral"}>{formatServiceAccountStatus(selectedServiceAccount.status)}</StatusChip>
                <button
                  type="button"
                  onClick={() => setSelectedServiceAccountID("")}
                  className="inline-flex h-7 w-7 items-center justify-center rounded-lg border border-stone-200 text-slate-400 transition hover:bg-stone-100 hover:text-slate-700"
                >
                  <X className="h-3.5 w-3.5" />
                </button>
              </div>
            </div>

            <div className="flex flex-wrap gap-2 border-b border-stone-200 px-5 py-3.5">
              <button
                type="button"
                onClick={() => issueCredentialMutation.mutate(selectedServiceAccount.id)}
                disabled={!csrfToken || issueCredentialMutation.isPending || selectedServiceAccount.status !== "active"}
                className="inline-flex h-8 items-center gap-1.5 rounded-lg border border-slate-900 bg-slate-900 px-3 text-xs font-medium text-white transition hover:bg-slate-800 disabled:cursor-not-allowed disabled:opacity-50"
              >
                {issueCredentialMutation.isPending ? <LoaderCircle className="h-3 w-3 animate-spin" /> : <KeyRound className="h-3 w-3" />}
                Issue credential
              </button>
              <button
                type="button"
                onClick={() => updateServiceAccountStatusMutation.mutate({ serviceAccountID: selectedServiceAccount.id, status: selectedServiceAccount.status === "active" ? "disabled" : "active" })}
                disabled={!csrfToken || updateServiceAccountStatusMutation.isPending}
                className="inline-flex h-8 items-center gap-1.5 rounded-lg border border-stone-200 bg-white px-3 text-xs font-medium text-slate-600 transition hover:bg-stone-100 disabled:cursor-not-allowed disabled:opacity-50"
              >
                <ShieldOff className="h-3 w-3" />
                {selectedServiceAccount.status === "active" ? "Disable" : "Enable"}
              </button>
            </div>

            {latestCredentialSecret && (
              <div className="mx-5 mt-4 rounded-lg border border-amber-200 bg-amber-50 px-3 py-3">
                <p className="text-xs font-semibold text-amber-800">Copy now — won&apos;t be shown again</p>
                <p className="mt-2 break-all rounded border border-amber-100 bg-white px-2 py-1.5 font-mono text-xs text-slate-800">{latestCredentialSecret.secret}</p>
                <button type="button" onClick={() => { void navigator.clipboard.writeText(latestCredentialSecret.secret); }} className="mt-2 inline-flex h-7 items-center gap-1.5 rounded border border-amber-200 bg-white px-2 text-xs text-amber-800 hover:bg-amber-100">
                  <Copy className="h-3 w-3" />
                  Copy
                </button>
              </div>
            )}

            <div className="mt-4 border-t border-stone-200">
              <div className="flex items-center justify-between px-5 py-3">
                <p className="text-xs font-semibold text-slate-700">Credentials</p>
                <PaginationControls page={pagedCredentials.page} totalPages={pagedCredentials.totalPages} onPageChange={setCredentialPage} label="Credentials" />
              </div>
              <div className="divide-y divide-stone-100">
                {pagedCredentials.items.length > 0 ? (
                  pagedCredentials.items.map((credential) => {
                    const isRevoked = Boolean(credential.revoked_at);
                    return (
                      <div key={credential.id} className="flex items-center gap-3 px-5 py-3">
                        <div className="min-w-0 flex-1">
                          <div className="flex items-center gap-2">
                            <p className="truncate text-xs font-medium text-slate-900">{credential.name}</p>
                            <StatusChip tone={isRevoked ? "danger" : "success"}>{isRevoked ? "Revoked" : "Active"}</StatusChip>
                          </div>
                          <p className="mt-0.5 text-[11px] text-slate-500">{describeCredentialActivity(credential)}</p>
                        </div>
                        {!isRevoked && (
                          <div className="flex shrink-0 gap-1">
                            <button type="button" onClick={() => rotateCredentialMutation.mutate({ serviceAccountID: selectedServiceAccount.id, credentialID: credential.id })} disabled={!csrfToken || rotateCredentialMutation.isPending} className="inline-flex h-7 items-center gap-1 rounded border border-stone-200 bg-white px-2 text-[11px] font-medium text-slate-600 transition hover:bg-stone-100 disabled:opacity-50">
                              <RefreshCw className="h-2.5 w-2.5" />
                              Rotate
                            </button>
                            <ConfirmDialog
                              title="Revoke this credential?"
                              description="This credential will immediately stop working. This action cannot be undone."
                              confirmLabel="Revoke credential"
                              onConfirm={async () => { await revokeCredentialMutation.mutateAsync({ serviceAccountID: selectedServiceAccount.id, credentialID: credential.id }); }}
                            >
                              {(open) => (
                                <button type="button" onClick={open} disabled={!csrfToken || revokeCredentialMutation.isPending} className="inline-flex h-7 items-center rounded border border-rose-200 bg-rose-50 px-2 text-[11px] font-medium text-rose-700 transition hover:bg-rose-100 disabled:opacity-50">Revoke</button>
                              )}
                            </ConfirmDialog>
                          </div>
                        )}
                      </div>
                    );
                  })
                ) : (
                  <p className="px-5 py-4 text-xs text-slate-500">No credentials issued yet.</p>
                )}
              </div>
            </div>
          </div>
        )}
      </div>
    </>
  );
}

/* ------------------------------------------------------------------ */
/*  Shared helpers (local)                                             */
/* ------------------------------------------------------------------ */

function StatusChip({
  tone,
  children,
}: {
  tone: "success" | "neutral" | "warning" | "danger" | "info";
  children: ReactNode;
}) {
  const toneClassName =
    tone === "success"
      ? "border-emerald-200 bg-emerald-50 text-emerald-700"
      : tone === "warning"
        ? "border-amber-200 bg-amber-50 text-amber-700"
        : tone === "danger"
          ? "border-rose-200 bg-rose-50 text-rose-700"
          : tone === "info"
            ? "border-sky-200 bg-sky-50 text-sky-700"
            : "border-stone-200 bg-stone-100 text-slate-700";

  return (
    <span className={`inline-flex h-7 items-center rounded-full border px-2.5 text-[11px] font-semibold ${toneClassName}`}>
      {children}
    </span>
  );
}

function PaginationControls({
  page,
  totalPages,
  onPageChange,
  label,
}: {
  page: number;
  totalPages: number;
  onPageChange: (page: number) => void;
  label: string;
}) {
  if (totalPages <= 1) {
    return null;
  }

  return (
    <div className="inline-flex items-center gap-2 rounded-lg border border-stone-200 bg-white px-2 py-2">
      <button
        type="button"
        onClick={() => onPageChange(page - 1)}
        disabled={page <= 1}
        aria-label={`Previous ${label} page`}
        className="inline-flex h-8 w-8 items-center justify-center rounded-lg border border-stone-200 text-slate-700 transition hover:bg-stone-100 disabled:cursor-not-allowed disabled:opacity-50"
      >
        <ChevronLeft className="h-4 w-4" />
      </button>
      <span className="min-w-[84px] text-center text-xs font-medium text-slate-500">
        Page {page} / {totalPages}
      </span>
      <button
        type="button"
        onClick={() => onPageChange(page + 1)}
        disabled={page >= totalPages}
        aria-label={`Next ${label} page`}
        className="inline-flex h-8 w-8 items-center justify-center rounded-lg border border-stone-200 text-slate-700 transition hover:bg-stone-100 disabled:cursor-not-allowed disabled:opacity-50"
      >
        <ChevronRight className="h-4 w-4" />
      </button>
    </div>
  );
}

function paginateItems<T>(items: T[], requestedPage: number, pageSize: number): { items: T[]; page: number; totalPages: number } {
  const totalPages = Math.max(1, Math.ceil(items.length / pageSize));
  const page = Math.min(Math.max(requestedPage, 1), totalPages);
  const start = (page - 1) * pageSize;
  return {
    items: items.slice(start, start + pageSize),
    page,
    totalPages,
  };
}

function formatServiceAccountRole(role: string): string {
  switch (role) {
    case "admin":
      return "Admin";
    case "writer":
      return "Writer";
    default:
      return "Reader";
  }
}

function formatServiceAccountStatus(status: string): string {
  return status === "active" ? "Active" : "Disabled";
}

function describeServiceAccountActivity(account: {
  credentials: Array<{ last_used_at?: string; revoked_at?: string }>;
  status: "active" | "disabled";
}): string {
  const lastUsed = account.credentials
    .map((credential) => credential.last_used_at)
    .filter((value): value is string => Boolean(value))
    .sort((left, right) => right.localeCompare(left))[0];

  if (lastUsed) {
    return `last used ${formatExactTimestamp(lastUsed)}`;
  }
  if (account.status === "disabled") {
    return "disabled";
  }
  return "no credential use recorded";
}

function describeLastUsed(account: { credentials: Array<{ last_used_at?: string }> }): string {
  const lastUsed = account.credentials
    ?.map(c => c.last_used_at)
    .filter((v): v is string => Boolean(v))
    .sort((a, b) => b.localeCompare(a))[0];
  return lastUsed ? formatExactTimestamp(lastUsed) : "Never";
}

function describeCredentialActivity(credential: { last_used_at?: string; revoked_at?: string }): string {
  if (credential.revoked_at) {
    return `Revoked ${formatExactTimestamp(credential.revoked_at)}`;
  }
  if (credential.last_used_at) {
    return `Last used ${formatExactTimestamp(credential.last_used_at)}`;
  }
  return "No usage recorded yet";
}
