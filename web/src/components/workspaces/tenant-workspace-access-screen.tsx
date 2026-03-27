"use client";

import { useEffect, useState } from "react";
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
import { type APIKeyAuditEvent } from "@/lib/types";

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
  const [selectedServiceAccountID, setSelectedServiceAccountID] = useState("");
  const [selectedAuditServiceAccountID, setSelectedAuditServiceAccountID] = useState("");
  const [selectedAuditEventID, setSelectedAuditEventID] = useState("");
  const [memberDraftRoles, setMemberDraftRoles] = useState<Record<string, "reader" | "writer" | "admin">>({});
  const [confirmingMemberAction, setConfirmingMemberAction] = useState<{ userID: string; action: "suspend" } | null>(null);

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
    onSuccess: async (_payload, input) => {
      setMemberDraftRoles((current) => {
        const next = { ...current };
        delete next[input.userID];
        return next;
      });
      setConfirmingMemberAction(null);
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
      setConfirmingMemberAction(null);
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
      setSelectedServiceAccountID(payload.service_account.id);
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
  const selectedServiceAccountIDValue = selectedServiceAccountID || serviceAccounts[0]?.id || "";
  const selectedServiceAccount =
    serviceAccounts.find((item) => item.id === selectedServiceAccountIDValue) ?? serviceAccounts[0] ?? null;
  const selectedAuditServiceAccountIDValue =
    selectedAuditServiceAccountID || serviceAccounts[0]?.id || "";
  const selectedAuditServiceAccount =
    serviceAccounts.find((item) => item.id === selectedAuditServiceAccountIDValue) ?? serviceAccounts[0] ?? null;
  const pendingInvitations = invitations.filter((item) => item.status === "pending");
  const latestInviteURL = createInvitationMutation.data?.accept_url ?? "";
  const currentUserID = session?.subject_id ?? "";
  const activeAdminCount = members.filter((member) => member.status === "active" && member.role === "admin").length;

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
  const selectedAuditEvent =
    (serviceAccountAuditQuery.data?.items ?? []).find((item) => item.id === selectedAuditEventID) ?? null;

  useEffect(() => {
    const items = serviceAccountAuditQuery.data?.items ?? [];
    if (items.length === 0) {
      setSelectedAuditEventID("");
      return;
    }
    if (selectedAuditEventID && !items.some((item) => item.id === selectedAuditEventID)) {
      setSelectedAuditEventID("");
    }
  }, [serviceAccountAuditQuery.data, selectedAuditEventID]);
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

  const isSelfMember = (userID: string): boolean => currentUserID !== "" && currentUserID === userID;
  const isLastActiveAdmin = (member: { role: string; status: string }): boolean =>
    member.status === "active" && member.role === "admin" && activeAdminCount <= 1;

  return (
    <div className="min-h-screen bg-[#f5f7fb] text-slate-900">
      <main className="mx-auto flex max-w-[1180px] flex-col gap-6 px-4 py-6 md:px-8 lg:px-10">
        <ControlPlaneNav />
        <AppBreadcrumbs items={[{ href: "/customers", label: "Workspace" }, { label: "Access" }]} />

        {!isAuthenticated ? <LoginRedirectNotice /> : null}
        {isAuthenticated && scope !== "tenant" ? (
          <ScopeNotice
            title="Workspace session required"
            body="Switch to a workspace session to manage members, invites, and service accounts."
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
              <p className="mt-3 text-sm text-slate-600">Manage people through membership and automation through service accounts.</p>
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
                    placeholder="What this credential is for"
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
                    <p className="text-[11px] uppercase tracking-[0.14em] text-emerald-700">New secret</p>
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
                    <p className="text-[11px] uppercase tracking-[0.14em] text-slate-500">New invite link</p>
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
                  <p className="text-xs uppercase tracking-[0.2em] text-slate-500">Service accounts</p>
                  <p className="mt-2 text-sm text-slate-600">Keep machine access scoped, rotated, and easy to review.</p>
                </div>
              </div>
              <div className="mt-4 grid gap-4 xl:grid-cols-[0.95fr_1.05fr]">
                <div className="grid gap-3">
                  {serviceAccounts.length > 0 ? (
                    serviceAccounts.map((account) => (
                      <button
                        key={account.id}
                        type="button"
                        onClick={() => setSelectedServiceAccountID(account.id)}
                        aria-pressed={selectedServiceAccountIDValue === account.id}
                        className={`rounded-2xl border px-4 py-4 text-left transition ${
                          selectedServiceAccountIDValue === account.id
                            ? "border-emerald-300 bg-emerald-50/60 shadow-sm"
                            : "border-stone-200 bg-stone-50 hover:border-stone-300 hover:bg-stone-100"
                        }`}
                      >
                        <div className="flex items-start justify-between gap-3">
                          <div className="min-w-0">
                            <p className="flex items-center gap-2 text-sm font-medium text-slate-950">
                              <ServerCog className="h-4 w-4 text-emerald-700" />
                              <span className="truncate">{account.name}</span>
                            </p>
                            <p className="mt-1 text-xs uppercase tracking-[0.14em] text-slate-500">
                              {formatServiceAccountRole(account.role)} · {formatServiceAccountStatus(account.status)} · {(account.environment || "unspecified").toUpperCase()}
                            </p>
                            <p className="mt-2 text-sm text-slate-700">
                              {account.active_credential_count} active credential{account.active_credential_count === 1 ? "" : "s"} · {describeServiceAccountActivity(account)}
                            </p>
                            {account.description ? <p className="mt-2 text-xs text-slate-500">{account.description}</p> : null}
                          </div>
                          <span className="rounded-full border border-stone-200 bg-white px-2.5 py-1 text-[11px] font-semibold uppercase tracking-[0.14em] text-slate-600">
                            {account.credentials.length} total
                          </span>
                        </div>
                      </button>
                    ))
                  ) : (
                    <p className="text-sm text-slate-500">No service accounts yet.</p>
                  )}
                </div>

                {selectedServiceAccount ? (
                  <div className="rounded-2xl border border-stone-200 bg-stone-50 px-4 py-4">
                    <div className="flex flex-col gap-3 border-b border-stone-200 pb-4 lg:flex-row lg:items-start lg:justify-between">
                      <div>
                        <p className="text-xs uppercase tracking-[0.14em] text-slate-500">Selected service account</p>
                        <p className="mt-2 text-lg font-semibold text-slate-950">{selectedServiceAccount.name}</p>
                        <p className="mt-2 text-sm text-slate-700">
                          {selectedServiceAccount.description || "Use this identity for a single automation or integration path."}
                        </p>
                        <p className="mt-2 text-xs text-slate-500">
                          {selectedServiceAccount.purpose || "No purpose recorded"} · created {formatExactTimestamp(selectedServiceAccount.created_at)}
                        </p>
                      </div>
                      <div className="flex flex-wrap gap-2">
                        <button
                          type="button"
                          onClick={() => issueCredentialMutation.mutate(selectedServiceAccount.id)}
                          disabled={!csrfToken || issueCredentialMutation.isPending || selectedServiceAccount.status !== "active"}
                          className="inline-flex h-10 items-center gap-2 rounded-xl border border-slate-900 bg-slate-900 px-3 text-xs uppercase tracking-[0.12em] text-white transition hover:bg-slate-800 disabled:cursor-not-allowed disabled:opacity-50"
                        >
                          {issueCredentialMutation.isPending ? <LoaderCircle className="h-3.5 w-3.5 animate-spin" /> : <KeyRound className="h-3.5 w-3.5" />}
                          Issue credential
                        </button>
                        <button
                          type="button"
                          onClick={() => setSelectedAuditServiceAccountID(selectedServiceAccount.id)}
                          className="inline-flex h-10 items-center gap-2 rounded-xl border border-stone-200 bg-white px-3 text-xs uppercase tracking-[0.12em] text-slate-700 transition hover:bg-stone-100"
                        >
                          <ShieldCheck className="h-3.5 w-3.5" />
                          Open audit
                        </button>
                        <button
                          type="button"
                          onClick={() =>
                            updateServiceAccountStatusMutation.mutate({
                              serviceAccountID: selectedServiceAccount.id,
                              status: selectedServiceAccount.status === "active" ? "disabled" : "active",
                            })
                          }
                          disabled={!csrfToken || updateServiceAccountStatusMutation.isPending}
                          className="inline-flex h-10 items-center gap-2 rounded-xl border border-stone-200 bg-white px-3 text-xs uppercase tracking-[0.12em] text-slate-700 transition hover:bg-stone-100 disabled:cursor-not-allowed disabled:opacity-50"
                        >
                          <ShieldOff className="h-3.5 w-3.5" />
                          {selectedServiceAccount.status === "active" ? "Disable" : "Enable"}
                        </button>
                      </div>
                    </div>

                    <div className="mt-4 grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
                      <DetailField label="Status" value={formatServiceAccountStatus(selectedServiceAccount.status)} />
                      <DetailField label="Access role" value={formatServiceAccountRole(selectedServiceAccount.role)} />
                      <DetailField label="Active credentials" value={`${selectedServiceAccount.active_credential_count}`} />
                      <DetailField label="Last activity" value={describeServiceAccountActivity(selectedServiceAccount)} />
                    </div>

                    <div className="mt-4">
                      <p className="text-xs uppercase tracking-[0.14em] text-slate-500">Current credentials</p>
                      <div className="mt-3 grid gap-3">
                        {selectedServiceAccount.credentials.length > 0 ? (
                          selectedServiceAccount.credentials.map((credential) => {
                            const isRevoked = Boolean(credential.revoked_at);
                            return (
                              <div key={credential.id} className="rounded-2xl border border-stone-200 bg-white px-4 py-4">
                                <div className="flex flex-col gap-3 lg:flex-row lg:items-center lg:justify-between">
                                  <div className="min-w-0">
                                    <div className="flex flex-wrap items-center gap-2">
                                      <p className="text-sm font-medium text-slate-950">{credential.name}</p>
                                      <span
                                        className={`rounded-full px-2.5 py-1 text-[11px] font-semibold uppercase tracking-[0.14em] ${
                                          isRevoked
                                            ? "border border-rose-200 bg-rose-50 text-rose-700"
                                            : "border border-emerald-200 bg-emerald-50 text-emerald-700"
                                        }`}
                                      >
                                        {isRevoked ? "Revoked" : "Active"}
                                      </span>
                                    </div>
                                    <p className="mt-1 text-xs text-slate-500">
                                      {credential.key_prefix} · issued {formatExactTimestamp(credential.created_at)}
                                    </p>
                                    <p className="mt-1 text-sm text-slate-700">{describeCredentialActivity(credential)}</p>
                                  </div>
                                  <div className="flex flex-wrap items-center gap-2">
                                    <button
                                      type="button"
                                      onClick={() =>
                                        rotateCredentialMutation.mutate({
                                          serviceAccountID: selectedServiceAccount.id,
                                          credentialID: credential.id,
                                        })
                                      }
                                      disabled={!csrfToken || isRevoked || rotateCredentialMutation.isPending}
                                      className="inline-flex h-10 items-center gap-2 rounded-xl border border-stone-200 bg-white px-3 text-xs uppercase tracking-[0.12em] text-slate-700 transition hover:bg-stone-100 disabled:cursor-not-allowed disabled:opacity-50"
                                    >
                                      <RefreshCw className="h-3.5 w-3.5" />
                                      Rotate
                                    </button>
                                    <button
                                      type="button"
                                      onClick={() =>
                                        revokeCredentialMutation.mutate({
                                          serviceAccountID: selectedServiceAccount.id,
                                          credentialID: credential.id,
                                        })
                                      }
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
                  </div>
                ) : (
                  <div className="rounded-2xl border border-dashed border-stone-300 bg-stone-50 px-4 py-6 text-sm text-slate-600">
                    Create a service account to review credential posture and machine access.
                  </div>
                )}
              </div>
            </section>

            <section className="rounded-3xl border border-stone-200 bg-white p-6 shadow-sm">
              <div className="flex flex-col gap-3 lg:flex-row lg:items-start lg:justify-between">
                <div>
                  <p className="text-xs uppercase tracking-[0.2em] text-slate-500">Credential audit</p>
                  <p className="mt-2 text-sm text-slate-600">Review credential lifecycle events and export evidence when needed.</p>
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
                  <p className="mt-1 text-sm text-slate-700">
                    {selectedAuditServiceAccount.active_credential_count} active credential{selectedAuditServiceAccount.active_credential_count === 1 ? "" : "s"} · {describeServiceAccountActivity(selectedAuditServiceAccount)}
                  </p>
                  <p className="mt-1 text-xs text-slate-500">
                    {formatServiceAccountRole(selectedAuditServiceAccount.role)} · {(selectedAuditServiceAccount.environment || "unspecified").toUpperCase()}
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
                          <ServiceAccountAuditRow
                            key={event.id}
                            event={event}
                            selected={event.id === selectedAuditEventID}
                            onSelect={() => setSelectedAuditEventID(event.id)}
                          />
                        ))
                      ) : (
                        <p className="text-sm text-slate-500">No audit events yet.</p>
                      )}
                    </div>
                  </div>
                  <div className="grid gap-4">
                    <ServiceAccountAuditDetail event={selectedAuditEvent} />
                    <div className="rounded-2xl border border-stone-200 bg-stone-50 px-4 py-4">
                      <p className="text-xs uppercase tracking-[0.14em] text-slate-500">Audit exports</p>
                      <div className="mt-3 grid gap-3">
                        {(serviceAccountAuditExportsQuery.data?.items ?? []).length > 0 ? (
                          (serviceAccountAuditExportsQuery.data?.items ?? []).map((item) => (
                            <div key={item.job.id} className="rounded-2xl border border-stone-200 bg-white px-4 py-3">
                              <div className="flex items-center justify-between gap-3">
                                <p className="text-sm font-medium text-slate-950">{formatAuditExportStatus(item.job.status)}</p>
                                <p className="text-xs text-slate-500">{formatExactTimestamp(item.job.created_at)}</p>
                              </div>
                              <p className="mt-2 text-sm text-slate-700">{item.job.row_count} row(s) prepared</p>
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
                        {(() => {
                          const draftRole = memberDraftRoles[member.user_id] ?? (member.role as "reader" | "writer" | "admin");
                          const roleDirty = draftRole !== member.role;
                          const selfMember = isSelfMember(member.user_id);
                          const lastAdminProtected = isLastActiveAdmin(member);
                          const showSuspendConfirm =
                            confirmingMemberAction?.userID === member.user_id && confirmingMemberAction.action === "suspend";
                          const roleSelectDisabled =
                            member.status !== "active" || updateMemberMutation.isPending || selfMember || lastAdminProtected;
                          const canApplyRole =
                            roleDirty && !roleSelectDisabled && Boolean(csrfToken);
                          const canSuspend =
                            member.status === "active" &&
                            Boolean(csrfToken) &&
                            !removeMemberMutation.isPending &&
                            !selfMember &&
                            !lastAdminProtected;
                          const canReactivate =
                            member.status !== "active" && Boolean(csrfToken) && !updateMemberMutation.isPending && !selfMember;

                          return (
                        <div className="flex flex-col gap-3 lg:flex-row lg:items-center lg:justify-between">
                          <div className="min-w-0">
                            <p className="flex items-center gap-2 text-sm font-medium text-slate-950">
                              <UserRound className="h-4 w-4 text-emerald-700" />
                              <span className="truncate">{member.display_name}</span>
                            </p>
                            <p className="mt-1 break-all text-xs text-slate-500">{member.email}</p>
                            <div className="mt-2 flex flex-wrap items-center gap-2">
                              <span className="rounded-full border border-stone-200 bg-white px-2.5 py-1 text-[11px] font-semibold uppercase tracking-[0.14em] text-slate-600">
                                {member.status}
                              </span>
                              {selfMember ? (
                                <span className="rounded-full border border-sky-200 bg-sky-50 px-2.5 py-1 text-[11px] font-semibold uppercase tracking-[0.14em] text-sky-700">
                                  You
                                </span>
                              ) : null}
                              {lastAdminProtected ? (
                                <span className="rounded-full border border-amber-200 bg-amber-50 px-2.5 py-1 text-[11px] font-semibold uppercase tracking-[0.14em] text-amber-700">
                                  Last active admin
                                </span>
                              ) : null}
                            </div>
                            {selfMember ? (
                              <p className="mt-2 text-xs text-slate-500">You cannot change your own membership from this screen.</p>
                            ) : null}
                            {lastAdminProtected ? (
                              <p className="mt-2 text-xs text-slate-500">Promote another active admin before changing this member&apos;s role or status.</p>
                            ) : null}
                          </div>
                          <div className="flex flex-wrap items-center gap-2">
                            <select
                              aria-label={`Role for ${member.email}`}
                              value={draftRole}
                              onChange={(event) =>
                                setMemberDraftRoles((current) => ({
                                  ...current,
                                  [member.user_id]: event.target.value as "reader" | "writer" | "admin",
                                }))
                              }
                              disabled={roleSelectDisabled}
                              className="h-10 rounded-xl border border-stone-200 bg-white px-3 text-xs uppercase tracking-[0.12em] text-slate-800 outline-none ring-slate-400 transition focus:ring-2"
                            >
                              <option value="admin">Admin</option>
                              <option value="writer">Writer</option>
                              <option value="reader">Reader</option>
                            </select>
                            {roleDirty ? (
                              <>
                                <button
                                  type="button"
                                  onClick={() => updateMemberMutation.mutate({ userID: member.user_id, role: draftRole })}
                                  disabled={!canApplyRole}
                                  className="inline-flex h-10 items-center gap-2 rounded-xl border border-slate-900 bg-slate-900 px-3 text-xs uppercase tracking-[0.12em] text-white transition hover:bg-slate-800 disabled:cursor-not-allowed disabled:opacity-50"
                                >
                                  Apply
                                </button>
                                <button
                                  type="button"
                                  onClick={() =>
                                    setMemberDraftRoles((current) => {
                                      const next = { ...current };
                                      delete next[member.user_id];
                                      return next;
                                    })
                                  }
                                  className="inline-flex h-10 items-center gap-2 rounded-xl border border-stone-200 bg-white px-3 text-xs uppercase tracking-[0.12em] text-slate-700 transition hover:bg-stone-100"
                                >
                                  Cancel
                                </button>
                              </>
                            ) : member.status === "active" ? (
                              showSuspendConfirm ? (
                                <>
                                  <button
                                    type="button"
                                    onClick={() => removeMemberMutation.mutate(member.user_id)}
                                    disabled={!canSuspend}
                                    className="inline-flex h-10 items-center gap-2 rounded-xl border border-rose-700 bg-rose-700 px-3 text-xs uppercase tracking-[0.12em] text-white transition hover:bg-rose-800 disabled:cursor-not-allowed disabled:opacity-50"
                                  >
                                    Confirm suspend
                                  </button>
                                  <button
                                    type="button"
                                    onClick={() => setConfirmingMemberAction(null)}
                                    className="inline-flex h-10 items-center gap-2 rounded-xl border border-stone-200 bg-white px-3 text-xs uppercase tracking-[0.12em] text-slate-700 transition hover:bg-stone-100"
                                  >
                                    Cancel
                                  </button>
                                </>
                              ) : (
                                <button
                                  type="button"
                                  onClick={() => setConfirmingMemberAction({ userID: member.user_id, action: "suspend" })}
                                  disabled={!canSuspend}
                                  className="inline-flex h-10 items-center gap-2 rounded-xl border border-rose-200 bg-rose-50 px-3 text-xs uppercase tracking-[0.12em] text-rose-700 transition hover:bg-rose-100 disabled:cursor-not-allowed disabled:opacity-50"
                                >
                                  <UserX className="h-3.5 w-3.5" />
                                  Suspend
                                </button>
                              )
                            ) : (
                              <button
                                type="button"
                                onClick={() => updateMemberMutation.mutate({ userID: member.user_id, role: member.role as "reader" | "writer" | "admin" })}
                                disabled={!canReactivate}
                                className="inline-flex h-10 items-center gap-2 rounded-xl border border-emerald-200 bg-emerald-50 px-3 text-xs uppercase tracking-[0.12em] text-emerald-700 transition hover:bg-emerald-100 disabled:cursor-not-allowed disabled:opacity-50"
                              >
                                Reactivate
                              </button>
                            )}
                          </div>
                        </div>
                          );
                        })()}
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

function ServiceAccountAuditRow({
  event,
  selected,
  onSelect,
}: {
  event: APIKeyAuditEvent;
  selected: boolean;
  onSelect: () => void;
}) {
  const presentation = describeAuditEvent(event);

  return (
    <button
      type="button"
      onClick={onSelect}
      aria-pressed={selected}
      aria-label={`View service account audit details for ${presentation.title}`}
      className={`w-full rounded-2xl border px-4 py-3 text-left transition ${
        selected
          ? "border-emerald-300 bg-emerald-50/60 shadow-sm"
          : "border-stone-200 bg-white hover:border-stone-300 hover:bg-stone-50"
      }`}
    >
      <div className="flex items-center justify-between gap-3">
        <p className="text-sm font-medium text-slate-950">{presentation.title}</p>
        <p className="text-xs text-slate-500">{formatExactTimestamp(event.created_at)}</p>
      </div>
      <p className="mt-2 text-sm text-slate-700">{presentation.summary}</p>
      <p className="mt-2 text-xs text-slate-500">{presentation.supporting}</p>
    </button>
  );
}

function ServiceAccountAuditDetail({ event }: { event: APIKeyAuditEvent | null }) {
  const entries = Object.entries(event?.metadata ?? {}).sort(([left], [right]) => left.localeCompare(right));

  if (!event) {
    return (
      <div className="rounded-2xl border border-dashed border-stone-300 bg-white px-4 py-6 text-sm text-slate-600">
        Select an audit event to inspect the full credential record.
      </div>
    );
  }

  return (
    <div className="rounded-2xl border border-stone-200 bg-white px-4 py-4">
      <p className="text-xs uppercase tracking-[0.14em] text-slate-500">Selected event</p>
      <p className="mt-2 text-lg font-semibold text-slate-950">{describeAuditEvent(event).title}</p>
      <p className="mt-2 text-sm text-slate-700">{describeAuditEvent(event).summary}</p>
      <div className="mt-3 grid gap-3 sm:grid-cols-2">
        <DetailField label="Event" value={describeAuditEvent(event).title} />
        <DetailField label="Created at" value={formatExactTimestamp(event.created_at)} />
        <DetailField label="Credential" value={event.api_key_id} mono />
        <DetailField label="Actor credential" value={event.actor_api_key_id || "Admin session"} mono />
        <DetailField label="Event ID" value={event.id} mono className="sm:col-span-2" />
      </div>
      {entries.length > 0 ? (
        <dl className="mt-3 grid gap-3 sm:grid-cols-2">
          {entries.map(([key, value]) => (
            <div key={key} className="rounded-lg border border-stone-200 bg-stone-50 px-3 py-3">
              <dt className="text-[11px] font-semibold uppercase tracking-[0.14em] text-slate-500">{formatAuditMetadataKey(key)}</dt>
              <dd className="mt-2 break-words text-sm text-slate-800">{formatAuditMetadataValue(value)}</dd>
            </div>
          ))}
        </dl>
      ) : (
        <p className="mt-3 text-sm text-slate-500">No metadata attached.</p>
      )}
    </div>
  );
}

function DetailField({
  label,
  value,
  mono,
  className = "",
}: {
  label: string;
  value: string;
  mono?: boolean;
  className?: string;
}) {
  return (
    <div className={`rounded-lg border border-stone-200 bg-stone-50 px-3 py-3 ${className}`.trim()}>
      <p className="text-[11px] font-semibold uppercase tracking-[0.14em] text-slate-500">{label}</p>
      <p className={`mt-2 break-words text-sm text-slate-900 ${mono ? "font-mono" : ""}`.trim()}>{value}</p>
    </div>
  );
}

function formatAuditMetadataValue(value: unknown): string {
  if (value === null || value === undefined) {
    return "-";
  }
  if (typeof value === "string" || typeof value === "number" || typeof value === "boolean") {
    return String(value);
  }
  try {
    return JSON.stringify(value);
  } catch {
    return String(value);
  }
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

function describeCredentialActivity(credential: { last_used_at?: string; revoked_at?: string }): string {
  if (credential.revoked_at) {
    return `Revoked ${formatExactTimestamp(credential.revoked_at)}`;
  }
  if (credential.last_used_at) {
    return `Last used ${formatExactTimestamp(credential.last_used_at)}`;
  }
  return "No usage recorded yet";
}

function formatServiceAccountRole(role: "reader" | "writer" | "admin"): string {
  switch (role) {
    case "admin":
      return "Admin";
    case "writer":
      return "Writer";
    default:
      return "Reader";
  }
}

function formatServiceAccountStatus(status: "active" | "disabled"): string {
  return status === "active" ? "Active" : "Disabled";
}

function describeAuditEvent(event: APIKeyAuditEvent): { title: string; summary: string; supporting: string } {
  const metadataCount = Object.keys(event.metadata ?? {}).length;
  const purpose = typeof event.metadata?.purpose === "string" ? event.metadata.purpose.trim() : "";
  const environment = typeof event.metadata?.environment === "string" ? event.metadata.environment.trim() : "";
  const context = [purpose, environment].filter(Boolean).join(" · ");
  const contextSuffix = context ? ` for ${context}` : "";
  const actorSummary = event.actor_api_key_id ? "Changed by another credential" : "Changed from the workspace access console";
  const metadataSummary =
    metadataCount > 0 ? `${metadataCount} supporting field${metadataCount === 1 ? "" : "s"}` : "No supporting fields";

  switch (event.action) {
    case "created":
      return {
        title: "Credential issued",
        summary: `A new credential was issued${contextSuffix}.`,
        supporting: `${actorSummary} · ${metadataSummary}`,
      };
    case "revoked":
      return {
        title: "Credential revoked",
        summary: `A credential was revoked${contextSuffix}.`,
        supporting: `${actorSummary} · ${metadataSummary}`,
      };
    case "rotated":
      return {
        title: "Credential rotated",
        summary: `A credential was rotated and replaced${contextSuffix}.`,
        supporting: `${actorSummary} · ${metadataSummary}`,
      };
    default:
      return {
        title: formatAuditActionLabel(event.action),
        summary: context ? `Credential activity recorded for ${context}.` : "Credential activity recorded.",
        supporting: `${actorSummary} · ${metadataSummary}`,
      };
  }
}

function formatAuditActionLabel(action: string): string {
  return action
    .split(/[_\s]+/)
    .filter(Boolean)
    .map((segment) => segment.charAt(0).toUpperCase() + segment.slice(1))
    .join(" ");
}

function formatAuditMetadataKey(key: string): string {
  return key
    .split("_")
    .filter(Boolean)
    .map((segment) => segment.charAt(0).toUpperCase() + segment.slice(1))
    .join(" ");
}

function formatAuditExportStatus(status: string): string {
  return status
    .split("_")
    .filter(Boolean)
    .map((segment) => segment.charAt(0).toUpperCase() + segment.slice(1))
    .join(" ");
}
