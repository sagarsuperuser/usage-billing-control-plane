"use client";

import { useState } from "react";
import {
  Copy,
  Download,
  KeyRound,
  LoaderCircle,
  MailPlus,
  RefreshCw,
  ServerCog,
  ShieldCheck,
  ShieldOff,
  UserRound,
  UserX,
} from "lucide-react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import { LoginRedirectNotice } from "@/components/auth/login-redirect-notice";
import { ScopeNotice } from "@/components/auth/scope-notice";
import { ControlPlaneNav } from "@/components/layout/control-plane-nav";
import { AppBreadcrumbs } from "@/components/layout/app-breadcrumbs";
import {
  createTenantWorkspaceInvitation,
  createTenantWorkspaceServiceAccountAuditExport,
  createTenantWorkspaceServiceAccount,
  fetchTenantWorkspaceServiceAccountAudit,
  fetchTenantWorkspaceServiceAccountAuditExports,
  fetchTenantWorkspaceInvitations,
  fetchTenantWorkspaceMembers,
  fetchTenantWorkspaceServiceAccounts,
  issueTenantWorkspaceServiceAccountCredential,
  removeTenantWorkspaceMember,
  revokeTenantWorkspaceInvitation,
  revokeTenantWorkspaceServiceAccountCredential,
  rotateTenantWorkspaceServiceAccountCredential,
  updateTenantWorkspaceMember,
  updateTenantWorkspaceServiceAccountStatus,
} from "@/lib/api";
import { formatExactTimestamp } from "@/lib/format";
import { useUISession } from "@/hooks/use-ui-session";

export function TenantWorkspaceAccessScreen() {
  const queryClient = useQueryClient();
  const { apiBaseURL, csrfToken, isAuthenticated, scope, role, isAdmin, session } = useUISession();
  const [inviteEmail, setInviteEmail] = useState("");
  const [inviteRole, setInviteRole] = useState<"reader" | "writer" | "admin">("writer");
  const [serviceAccountName, setServiceAccountName] = useState("");
  const [serviceAccountDescription, setServiceAccountDescription] = useState("");
  const [serviceAccountRole, setServiceAccountRole] = useState<"reader" | "writer" | "admin">("writer");
  const [serviceAccountPurpose, setServiceAccountPurpose] = useState("");
  const [serviceAccountEnvironment, setServiceAccountEnvironment] = useState("prod");
  const [latestCredentialSecret, setLatestCredentialSecret] = useState<{ label: string; secret: string } | null>(null);
  const [selectedAuditServiceAccountID, setSelectedAuditServiceAccountID] = useState("");

  const workspaceQueryKey = ["tenant-workspace-members", apiBaseURL, session?.tenant_id];
  const invitationQueryKey = ["tenant-workspace-invitations", apiBaseURL, session?.tenant_id];
  const serviceAccountQueryKey = ["tenant-workspace-service-accounts", apiBaseURL, session?.tenant_id];

  const membersQuery = useQuery({
    queryKey: workspaceQueryKey,
    queryFn: () => fetchTenantWorkspaceMembers({ runtimeBaseURL: apiBaseURL }),
    enabled: isAuthenticated && scope === "tenant" && isAdmin,
  });
  const invitationsQuery = useQuery({
    queryKey: invitationQueryKey,
    queryFn: () => fetchTenantWorkspaceInvitations({ runtimeBaseURL: apiBaseURL }),
    enabled: isAuthenticated && scope === "tenant" && isAdmin,
  });
  const serviceAccountsQuery = useQuery({
    queryKey: serviceAccountQueryKey,
    queryFn: () => fetchTenantWorkspaceServiceAccounts({ runtimeBaseURL: apiBaseURL }),
    enabled: isAuthenticated && scope === "tenant" && isAdmin,
  });

  const createInvitationMutation = useMutation({
    mutationFn: () =>
      createTenantWorkspaceInvitation({
        runtimeBaseURL: apiBaseURL,
        csrfToken,
        email: inviteEmail,
        role: inviteRole,
      }),
    onSuccess: async () => {
      setInviteEmail("");
      await queryClient.invalidateQueries({ queryKey: invitationQueryKey });
    },
  });
  const revokeInvitationMutation = useMutation({
    mutationFn: (invitationID: string) =>
      revokeTenantWorkspaceInvitation({
        runtimeBaseURL: apiBaseURL,
        csrfToken,
        invitationID,
      }),
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: invitationQueryKey });
    },
  });
  const updateMemberMutation = useMutation({
    mutationFn: (input: { userID: string; role: "reader" | "writer" | "admin" }) =>
      updateTenantWorkspaceMember({
        runtimeBaseURL: apiBaseURL,
        csrfToken,
        userID: input.userID,
        role: input.role,
      }),
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: workspaceQueryKey });
    },
  });
  const removeMemberMutation = useMutation({
    mutationFn: (userID: string) =>
      removeTenantWorkspaceMember({
        runtimeBaseURL: apiBaseURL,
        csrfToken,
        userID,
      }),
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: workspaceQueryKey });
    },
  });
  const createServiceAccountMutation = useMutation({
    mutationFn: () =>
      createTenantWorkspaceServiceAccount({
        runtimeBaseURL: apiBaseURL,
        csrfToken,
        name: serviceAccountName,
        description: serviceAccountDescription,
        role: serviceAccountRole,
        purpose: serviceAccountPurpose,
        environment: serviceAccountEnvironment,
        issueInitialCredential: true,
      }),
    onSuccess: async (payload) => {
      setServiceAccountName("");
      setServiceAccountDescription("");
      setServiceAccountPurpose("");
      setServiceAccountEnvironment("prod");
      setSelectedAuditServiceAccountID(payload.service_account.id);
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
      await queryClient.invalidateQueries({
        queryKey: ["tenant-workspace-service-account-audit", apiBaseURL, session?.tenant_id, selectedAuditServiceAccountIDValue],
      });
    },
  });

  const members = membersQuery.data ?? [];
  const invitations = invitationsQuery.data ?? [];
  const serviceAccounts = serviceAccountsQuery.data ?? [];
  const selectedAuditServiceAccountIDValue =
    selectedAuditServiceAccountID || serviceAccounts[0]?.id || "";
  const selectedAuditServiceAccount =
    serviceAccounts.find((item) => item.id === selectedAuditServiceAccountIDValue) ?? serviceAccounts[0] ?? null;
  const pendingInvitations = invitations.filter((item) => item.status === "pending");
  const latestInviteURL = createInvitationMutation.data?.accept_url ?? "";

  const serviceAccountAuditQuery = useQuery({
    queryKey: ["tenant-workspace-service-account-audit", apiBaseURL, session?.tenant_id, selectedAuditServiceAccountIDValue],
    queryFn: () =>
      fetchTenantWorkspaceServiceAccountAudit({
        runtimeBaseURL: apiBaseURL,
        serviceAccountID: selectedAuditServiceAccountIDValue,
        limit: 10,
      }),
    enabled: isAuthenticated && scope === "tenant" && isAdmin && selectedAuditServiceAccountIDValue !== "",
  });
  const serviceAccountAuditExportsQuery = useQuery({
    queryKey: ["tenant-workspace-service-account-audit-exports", apiBaseURL, session?.tenant_id, selectedAuditServiceAccountIDValue],
    queryFn: () =>
      fetchTenantWorkspaceServiceAccountAuditExports({
        runtimeBaseURL: apiBaseURL,
        serviceAccountID: selectedAuditServiceAccountIDValue,
        limit: 5,
      }),
    enabled: isAuthenticated && scope === "tenant" && isAdmin && selectedAuditServiceAccountIDValue !== "",
  });
  const createAuditExportMutation = useMutation({
    mutationFn: () =>
      createTenantWorkspaceServiceAccountAuditExport({
        runtimeBaseURL: apiBaseURL,
        csrfToken,
        serviceAccountID: selectedAuditServiceAccountIDValue,
        idempotencyKey: `svcacct-${selectedAuditServiceAccountIDValue}-${Date.now()}`,
      }),
    onSuccess: async () => {
      await queryClient.invalidateQueries({
        queryKey: ["tenant-workspace-service-account-audit-exports", apiBaseURL, session?.tenant_id, selectedAuditServiceAccountIDValue],
      });
    },
  });

  return (
    <div className="min-h-screen bg-[#f5f7fb] text-slate-900">
      <main className="mx-auto flex max-w-[1180px] flex-col gap-6 px-4 py-6 md:px-8 lg:px-10">
        <ControlPlaneNav />
        <AppBreadcrumbs items={[{ href: "/customers", label: "Workspace" }, { label: "Access" }]} />

        {!isAuthenticated ? <LoginRedirectNotice /> : null}
        {isAuthenticated && scope !== "tenant" ? (
          <ScopeNotice
            title="Tenant session required"
            body="Workspace access management belongs inside a tenant workspace. Switch to a tenant session to manage members, invitations, and machine credentials."
            actionHref="/billing-connections"
            actionLabel="Open platform home"
          />
        ) : null}
        {isAuthenticated && scope === "tenant" && !isAdmin ? (
          <ScopeNotice
            title="Workspace admin role required"
            body={`You are signed in as ${role || "reader"}. Only workspace admins can manage invitations, service accounts, roles, and member removal.`}
            actionHref="/customers"
            actionLabel="Open workspace home"
          />
        ) : null}

        {isAuthenticated && scope === "tenant" && isAdmin ? (
          <>
            <section className="rounded-3xl border border-stone-200 bg-white p-6 shadow-sm">
              <p className="text-xs uppercase tracking-[0.2em] text-slate-500">Workspace access</p>
              <h1 className="mt-2 text-3xl font-semibold text-slate-950">Members, invitations, and machine credentials</h1>
              <p className="mt-3 text-sm text-slate-600">
                Tenant admins own workspace access after platform handoff. Human access stays on membership; machine access should move through named service accounts.
              </p>
            </section>

            <section className="grid gap-4 xl:grid-cols-[1.1fr_0.9fr]">
              <div className="rounded-3xl border border-stone-200 bg-white p-6 shadow-sm">
                <p className="text-xs uppercase tracking-[0.2em] text-slate-500">Service accounts</p>
                <div className="mt-4 grid gap-3 md:grid-cols-2">
                  <input
                    type="text"
                    value={serviceAccountName}
                    onChange={(event) => setServiceAccountName(event.target.value)}
                    placeholder="Acme ERP Sync"
                    className="h-11 rounded-xl border border-stone-200 bg-white px-3 text-sm text-slate-900 outline-none ring-slate-400 transition focus:ring-2"
                  />
                  <select
                    aria-label="Service account role"
                    value={serviceAccountRole}
                    onChange={(event) => setServiceAccountRole(event.target.value as "reader" | "writer" | "admin")}
                    className="h-11 rounded-xl border border-stone-200 bg-white px-3 text-sm text-slate-900 outline-none ring-slate-400 transition focus:ring-2"
                  >
                    <option value="admin">Admin</option>
                    <option value="writer">Writer</option>
                    <option value="reader">Reader</option>
                  </select>
                  <input
                    type="text"
                    value={serviceAccountPurpose}
                    onChange={(event) => setServiceAccountPurpose(event.target.value)}
                    placeholder="erp-sync"
                    className="h-11 rounded-xl border border-stone-200 bg-white px-3 text-sm text-slate-900 outline-none ring-slate-400 transition focus:ring-2"
                  />
                  <input
                    type="text"
                    value={serviceAccountEnvironment}
                    onChange={(event) => setServiceAccountEnvironment(event.target.value)}
                    placeholder="prod"
                    className="h-11 rounded-xl border border-stone-200 bg-white px-3 text-sm text-slate-900 outline-none ring-slate-400 transition focus:ring-2"
                  />
                  <textarea
                    value={serviceAccountDescription}
                    onChange={(event) => setServiceAccountDescription(event.target.value)}
                    placeholder="Describe the automation that will own this credential"
                    rows={3}
                    className="md:col-span-2 rounded-xl border border-stone-200 bg-white px-3 py-3 text-sm text-slate-900 outline-none ring-slate-400 transition focus:ring-2"
                  />
                  <button
                    type="button"
                    onClick={() => createServiceAccountMutation.mutate()}
                    disabled={!csrfToken || !serviceAccountName.trim() || createServiceAccountMutation.isPending}
                    className="md:col-span-2 inline-flex h-11 items-center justify-center gap-2 rounded-xl border border-slate-900 bg-slate-900 px-4 text-sm font-medium text-white transition hover:bg-slate-800 disabled:cursor-not-allowed disabled:opacity-50"
                  >
                    {createServiceAccountMutation.isPending ? <LoaderCircle className="h-4 w-4 animate-spin" /> : <ServerCog className="h-4 w-4" />}
                    Create and issue first credential
                  </button>
                </div>
                {latestCredentialSecret ? (
                  <div className="mt-4 rounded-2xl border border-emerald-200 bg-emerald-50 px-4 py-3">
                    <p className="text-[11px] uppercase tracking-[0.14em] text-emerald-700">Latest credential secret</p>
                    <p className="mt-2 text-xs font-medium text-slate-800">{latestCredentialSecret.label}</p>
                    <p className="mt-2 break-all font-mono text-xs text-slate-700">{latestCredentialSecret.secret}</p>
                    <button
                      type="button"
                      onClick={() => {
                        void navigator.clipboard.writeText(latestCredentialSecret.secret);
                      }}
                      className="mt-3 inline-flex h-9 items-center gap-2 rounded-xl border border-emerald-200 bg-white px-3 text-xs text-emerald-700 transition hover:bg-emerald-100"
                    >
                      <Copy className="h-3.5 w-3.5" />
                      Copy secret
                    </button>
                  </div>
                ) : null}
              </div>

              <div className="rounded-3xl border border-stone-200 bg-white p-6 shadow-sm">
                <p className="text-xs uppercase tracking-[0.2em] text-slate-500">Invite member</p>
                <div className="mt-4 grid gap-3">
                  <input
                    type="email"
                    value={inviteEmail}
                    onChange={(event) => setInviteEmail(event.target.value)}
                    placeholder="teammate@example.com"
                    className="h-11 rounded-xl border border-stone-200 bg-white px-3 text-sm text-slate-900 outline-none ring-slate-400 transition focus:ring-2"
                  />
                  <select
                    aria-label="Workspace role"
                    value={inviteRole}
                    onChange={(event) => setInviteRole(event.target.value as "reader" | "writer" | "admin")}
                    className="h-11 rounded-xl border border-stone-200 bg-white px-3 text-sm text-slate-900 outline-none ring-slate-400 transition focus:ring-2"
                  >
                    <option value="admin">Admin</option>
                    <option value="writer">Writer</option>
                    <option value="reader">Reader</option>
                  </select>
                  <button
                    type="button"
                    onClick={() => createInvitationMutation.mutate()}
                    disabled={!csrfToken || !inviteEmail.trim() || createInvitationMutation.isPending}
                    className="inline-flex h-11 items-center justify-center gap-2 rounded-xl border border-slate-900 bg-slate-900 px-4 text-sm font-medium text-white transition hover:bg-slate-800 disabled:cursor-not-allowed disabled:opacity-50"
                  >
                    {createInvitationMutation.isPending ? <LoaderCircle className="h-4 w-4 animate-spin" /> : <MailPlus className="h-4 w-4" />}
                    Send invite
                  </button>
                </div>
                {latestInviteURL ? (
                  <div className="mt-4 rounded-2xl border border-stone-200 bg-stone-50 px-4 py-3">
                    <p className="text-[11px] uppercase tracking-[0.14em] text-slate-500">Latest invite link</p>
                    <p className="mt-2 break-all text-xs text-slate-700">{latestInviteURL}</p>
                    <button
                      type="button"
                      onClick={() => {
                        void navigator.clipboard.writeText(latestInviteURL);
                      }}
                      className="mt-3 inline-flex h-9 items-center gap-2 rounded-xl border border-stone-200 bg-white px-3 text-xs text-slate-700 transition hover:bg-stone-100"
                    >
                      <Copy className="h-3.5 w-3.5" />
                      Copy invite link
                    </button>
                  </div>
                ) : null}
              </div>
            </section>

            <section className="rounded-3xl border border-stone-200 bg-white p-6 shadow-sm">
              <div className="flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
                <div>
                  <p className="text-xs uppercase tracking-[0.2em] text-slate-500">Current service accounts</p>
                  <p className="mt-2 text-sm text-slate-600">Each machine identity owns one or more API credentials. Rotate credentials without losing the machine identity record.</p>
                </div>
              </div>
              <div className="mt-4 grid gap-4">
                {serviceAccounts.length > 0 ? (
                  serviceAccounts.map((account) => (
                    <div key={account.id} className="rounded-2xl border border-stone-200 bg-stone-50 px-4 py-4">
                      <div className="flex flex-col gap-3 border-b border-stone-200 pb-4 lg:flex-row lg:items-start lg:justify-between">
                        <div>
                          <p className="flex items-center gap-2 text-sm font-medium text-slate-950">
                            <ServerCog className="h-4 w-4 text-emerald-700" />
                            {account.name}
                          </p>
                          <p className="mt-1 text-xs uppercase tracking-[0.14em] text-slate-500">{account.role} · {account.status} · {account.environment || "unspecified"} · {account.active_credential_count} active credential(s)</p>
                          {account.description ? <p className="mt-2 text-sm text-slate-600">{account.description}</p> : null}
                          {account.purpose ? <p className="mt-2 text-xs text-slate-500">Purpose: {account.purpose}</p> : null}
                        </div>
                        <div className="flex flex-wrap gap-2">
                          <button
                            type="button"
                            onClick={() => issueCredentialMutation.mutate(account.id)}
                            disabled={!csrfToken || issueCredentialMutation.isPending || account.status !== "active"}
                            className="inline-flex h-10 items-center gap-2 rounded-xl border border-stone-200 bg-white px-3 text-xs uppercase tracking-[0.12em] text-slate-700 transition hover:bg-stone-100 disabled:cursor-not-allowed disabled:opacity-50"
                          >
                            {issueCredentialMutation.isPending ? <LoaderCircle className="h-3.5 w-3.5 animate-spin" /> : <KeyRound className="h-3.5 w-3.5" />}
                            Issue credential
                          </button>
                          <button
                            type="button"
                            onClick={() => setSelectedAuditServiceAccountID(account.id)}
                            className="inline-flex h-10 items-center gap-2 rounded-xl border border-stone-200 bg-white px-3 text-xs uppercase tracking-[0.12em] text-slate-700 transition hover:bg-stone-100"
                          >
                            <ShieldCheck className="h-3.5 w-3.5" />
                            View audit
                          </button>
                          <button
                            type="button"
                            onClick={() =>
                              updateServiceAccountStatusMutation.mutate({
                                serviceAccountID: account.id,
                                status: account.status === "active" ? "disabled" : "active",
                              })
                            }
                            disabled={!csrfToken || updateServiceAccountStatusMutation.isPending}
                            className="inline-flex h-10 items-center gap-2 rounded-xl border border-stone-200 bg-white px-3 text-xs uppercase tracking-[0.12em] text-slate-700 transition hover:bg-stone-100 disabled:cursor-not-allowed disabled:opacity-50"
                          >
                            <ShieldOff className="h-3.5 w-3.5" />
                            {account.status === "active" ? "Disable" : "Enable"}
                          </button>
                        </div>
                      </div>
                      <div className="mt-4 grid gap-3">
                        {account.credentials.length > 0 ? (
                          account.credentials.map((credential) => {
                            const isRevoked = Boolean(credential.revoked_at);
                            return (
                              <div key={credential.id} className="rounded-2xl border border-stone-200 bg-white px-4 py-3">
                                <div className="flex flex-col gap-3 lg:flex-row lg:items-center lg:justify-between">
                                  <div className="min-w-0">
                                    <p className="text-sm font-medium text-slate-950">{credential.name}</p>
                                    <p className="mt-1 text-xs text-slate-500">{credential.key_prefix} · created {formatExactTimestamp(credential.created_at)}</p>
                                    <p className="mt-1 text-xs text-slate-500">{isRevoked ? `revoked ${formatExactTimestamp(credential.revoked_at!)}` : credential.last_used_at ? `last used ${formatExactTimestamp(credential.last_used_at)}` : "never used"}</p>
                                  </div>
                                  <div className="flex flex-wrap items-center gap-2">
                                    <button
                                      type="button"
                                      onClick={() => rotateCredentialMutation.mutate({ serviceAccountID: account.id, credentialID: credential.id })}
                                      disabled={!csrfToken || isRevoked || rotateCredentialMutation.isPending}
                                      className="inline-flex h-10 items-center gap-2 rounded-xl border border-stone-200 bg-white px-3 text-xs uppercase tracking-[0.12em] text-slate-700 transition hover:bg-stone-100 disabled:cursor-not-allowed disabled:opacity-50"
                                    >
                                      <RefreshCw className="h-3.5 w-3.5" />
                                      Rotate
                                    </button>
                                    <button
                                      type="button"
                                      onClick={() => revokeCredentialMutation.mutate({ serviceAccountID: account.id, credentialID: credential.id })}
                                      disabled={!csrfToken || isRevoked || revokeCredentialMutation.isPending}
                                      className="inline-flex h-10 items-center gap-2 rounded-xl border border-rose-200 bg-rose-50 px-3 text-xs uppercase tracking-[0.12em] text-rose-700 transition hover:bg-rose-100 disabled:cursor-not-allowed disabled:opacity-50"
                                    >
                                      <UserX className="h-3.5 w-3.5" />
                                      Revoke
                                    </button>
                                  </div>
                                </div>
                              </div>
                            );
                          })
                        ) : (
                          <p className="text-sm text-slate-500">No credentials issued yet.</p>
                        )}
                      </div>
                    </div>
                  ))
                ) : (
                  <p className="text-sm text-slate-500">No service accounts yet.</p>
                )}
              </div>
            </section>

            <section className="rounded-3xl border border-stone-200 bg-white p-6 shadow-sm">
              <div className="flex flex-col gap-3 lg:flex-row lg:items-start lg:justify-between">
                <div>
                  <p className="text-xs uppercase tracking-[0.2em] text-slate-500">Credential audit</p>
                  <p className="mt-2 text-sm text-slate-600">
                    Audit stays attached to the machine identity. Review recent credential activity and export a CSV for compliance or incident response.
                  </p>
                </div>
                <div className="flex flex-wrap gap-2">
                  <select
                    aria-label="Audit service account"
                    value={selectedAuditServiceAccountIDValue}
                    onChange={(event) => setSelectedAuditServiceAccountID(event.target.value)}
                    className="h-10 rounded-xl border border-stone-200 bg-white px-3 text-xs uppercase tracking-[0.12em] text-slate-800 outline-none ring-slate-400 transition focus:ring-2"
                  >
                    {serviceAccounts.map((account) => (
                      <option key={account.id} value={account.id}>
                        {account.name}
                      </option>
                    ))}
                  </select>
                  <button
                    type="button"
                    onClick={() => createAuditExportMutation.mutate()}
                    disabled={!csrfToken || !selectedAuditServiceAccountIDValue || createAuditExportMutation.isPending}
                    className="inline-flex h-10 items-center gap-2 rounded-xl border border-stone-200 bg-white px-3 text-xs uppercase tracking-[0.12em] text-slate-700 transition hover:bg-stone-100 disabled:cursor-not-allowed disabled:opacity-50"
                  >
                    {createAuditExportMutation.isPending ? <LoaderCircle className="h-3.5 w-3.5 animate-spin" /> : <Download className="h-3.5 w-3.5" />}
                    Export audit CSV
                  </button>
                </div>
              </div>
              {selectedAuditServiceAccount ? (
                <div className="mt-4 rounded-2xl border border-stone-200 bg-stone-50 px-4 py-4">
                  <p className="text-sm font-medium text-slate-950">{selectedAuditServiceAccount.name}</p>
                  <p className="mt-1 text-xs text-slate-500">
                    {selectedAuditServiceAccount.role} · {selectedAuditServiceAccount.environment || "unspecified"} · {selectedAuditServiceAccount.active_credential_count} active credential(s)
                  </p>
                </div>
              ) : (
                <p className="mt-4 text-sm text-slate-500">Create a service account first to inspect machine-credential audit.</p>
              )}
              {selectedAuditServiceAccount ? (
                <div className="mt-4 grid gap-4 xl:grid-cols-[1.1fr_0.9fr]">
                  <div className="rounded-2xl border border-stone-200 bg-stone-50 px-4 py-4">
                    <p className="text-xs uppercase tracking-[0.14em] text-slate-500">Recent events</p>
                    <div className="mt-3 grid gap-3">
                      {(serviceAccountAuditQuery.data?.items ?? []).length > 0 ? (
                        (serviceAccountAuditQuery.data?.items ?? []).map((event) => (
                          <div key={event.id} className="rounded-2xl border border-stone-200 bg-white px-4 py-3">
                            <div className="flex items-center justify-between gap-3">
                              <p className="text-sm font-medium text-slate-950">{event.action}</p>
                              <p className="text-xs text-slate-500">{formatExactTimestamp(event.created_at)}</p>
                            </div>
                            <p className="mt-2 text-xs text-slate-500">{event.api_key_id}</p>
                          </div>
                        ))
                      ) : (
                        <p className="text-sm text-slate-500">No audit events yet.</p>
                      )}
                    </div>
                  </div>
                  <div className="rounded-2xl border border-stone-200 bg-stone-50 px-4 py-4">
                    <p className="text-xs uppercase tracking-[0.14em] text-slate-500">Audit exports</p>
                    <div className="mt-3 grid gap-3">
                      {(serviceAccountAuditExportsQuery.data?.items ?? []).length > 0 ? (
                        (serviceAccountAuditExportsQuery.data?.items ?? []).map((item) => (
                          <div key={item.job.id} className="rounded-2xl border border-stone-200 bg-white px-4 py-3">
                            <div className="flex items-center justify-between gap-3">
                              <p className="text-sm font-medium text-slate-950">{item.job.status}</p>
                              <p className="text-xs text-slate-500">{formatExactTimestamp(item.job.created_at)}</p>
                            </div>
                            <p className="mt-2 text-xs text-slate-500">{item.job.row_count} row(s)</p>
                            {item.download_url ? (
                              <a
                                href={item.download_url}
                                target="_blank"
                                rel="noreferrer"
                                className="mt-3 inline-flex h-9 items-center gap-2 rounded-xl border border-stone-200 bg-white px-3 text-xs text-slate-700 transition hover:bg-stone-100"
                              >
                                <Download className="h-3.5 w-3.5" />
                                Download CSV
                              </a>
                            ) : null}
                          </div>
                        ))
                      ) : (
                        <p className="text-sm text-slate-500">No audit exports created yet.</p>
                      )}
                    </div>
                  </div>
                </div>
              ) : null}
            </section>

            <section className="grid gap-4 md:grid-cols-2">
              <div className="rounded-3xl border border-stone-200 bg-white p-6 shadow-sm">
                <p className="text-xs uppercase tracking-[0.2em] text-slate-500">Pending invites</p>
                <div className="mt-4 grid gap-3">
                  {pendingInvitations.length > 0 ? (
                    pendingInvitations.map((invite) => (
                      <div key={invite.id} className="rounded-2xl border border-stone-200 bg-stone-50 px-4 py-3">
                        <div className="flex items-start justify-between gap-3">
                          <div className="min-w-0">
                            <p className="flex items-center gap-2 text-sm font-medium text-slate-950">
                              <ShieldCheck className="h-4 w-4 text-amber-600" />
                              <span className="truncate">{invite.email}</span>
                            </p>
                            <p className="mt-1 text-xs text-slate-500">
                              {invite.role} · expires {formatExactTimestamp(invite.expires_at)}
                            </p>
                            {invite.accept_url ? <p className="mt-2 break-all text-[11px] text-slate-500">{invite.accept_url}</p> : null}
                          </div>
                          <button
                            type="button"
                            onClick={() => revokeInvitationMutation.mutate(invite.id)}
                            disabled={!csrfToken || revokeInvitationMutation.isPending}
                            className="inline-flex h-9 items-center justify-center rounded-xl border border-stone-200 bg-white px-3 text-xs text-slate-700 transition hover:bg-stone-100 disabled:cursor-not-allowed disabled:opacity-50"
                          >
                            Revoke
                          </button>
                        </div>
                      </div>
                    ))
                  ) : (
                    <p className="text-sm text-slate-500">No pending workspace invites.</p>
                  )}
                </div>
              </div>

              <div className="rounded-3xl border border-stone-200 bg-white p-6 shadow-sm">
                <p className="text-xs uppercase tracking-[0.2em] text-slate-500">Current members</p>
                <div className="mt-4 grid gap-3">
                  {members.length > 0 ? (
                    members.map((member) => (
                      <div key={member.user_id} className="rounded-2xl border border-stone-200 bg-stone-50 px-4 py-4">
                        <div className="flex flex-col gap-3 lg:flex-row lg:items-center lg:justify-between">
                          <div className="min-w-0">
                            <p className="flex items-center gap-2 text-sm font-medium text-slate-950">
                              <UserRound className="h-4 w-4 text-emerald-700" />
                              <span className="truncate">{member.display_name}</span>
                            </p>
                            <p className="mt-1 break-all text-xs text-slate-500">{member.email}</p>
                          </div>
                          <div className="flex flex-wrap items-center gap-2">
                            <select
                              aria-label={`Role for ${member.email}`}
                              defaultValue={member.role}
                              onChange={(event) =>
                                updateMemberMutation.mutate({
                                  userID: member.user_id,
                                  role: event.target.value as "reader" | "writer" | "admin",
                                })
                              }
                              disabled={!csrfToken || updateMemberMutation.isPending}
                              className="h-10 rounded-xl border border-stone-200 bg-white px-3 text-xs uppercase tracking-[0.12em] text-slate-800 outline-none ring-slate-400 transition focus:ring-2"
                            >
                              <option value="admin">Admin</option>
                              <option value="writer">Writer</option>
                              <option value="reader">Reader</option>
                            </select>
                            <button
                              type="button"
                              onClick={() => removeMemberMutation.mutate(member.user_id)}
                              disabled={!csrfToken || removeMemberMutation.isPending}
                              className="inline-flex h-10 items-center gap-2 rounded-xl border border-rose-200 bg-rose-50 px-3 text-xs uppercase tracking-[0.12em] text-rose-700 transition hover:bg-rose-100 disabled:cursor-not-allowed disabled:opacity-50"
                            >
                              <UserX className="h-3.5 w-3.5" />
                              Remove
                            </button>
                          </div>
                        </div>
                      </div>
                    ))
                  ) : (
                    <p className="text-sm text-slate-500">No active members yet.</p>
                  )}
                </div>
              </div>
            </section>
          </>
        ) : null}
      </main>
    </div>
  );
}
