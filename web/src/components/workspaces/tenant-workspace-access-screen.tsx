"use client";

import { useRef, useState } from "react";
import {
  ChevronLeft,
  ChevronRight,
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
import { AppBreadcrumbs } from "@/components/layout/app-breadcrumbs";
import { ControlPlaneNav } from "@/components/layout/control-plane-nav";
import { useUISession } from "@/hooks/use-ui-session";
import {
  createTenantWorkspaceInvitation,
  createTenantWorkspaceServiceAccount,
  fetchTenantWorkspaceInvitations,
  fetchTenantWorkspaceMembers,
  fetchTenantWorkspaceServiceAccountAudit,
  fetchTenantWorkspaceServiceAccountAuditExports,
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
import { type APIKeyAuditEvent } from "@/lib/types";

export function TenantWorkspaceAccessScreen() {
  const queryClient = useQueryClient();
  const { apiBaseURL, csrfToken, isAuthenticated, scope, role, isAdmin, session } = useUISession();
  const peopleSectionRef = useRef<HTMLElement | null>(null);
  const machineSectionRef = useRef<HTMLElement | null>(null);
  const auditSectionRef = useRef<HTMLElement | null>(null);
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
  const [memberPage, setMemberPage] = useState(1);
  const [invitePage, setInvitePage] = useState(1);
  const [serviceAccountPage, setServiceAccountPage] = useState(1);
  const [credentialPage, setCredentialPage] = useState(1);

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
  const selectedAuditServiceAccountIDValue = selectedAuditServiceAccountID || serviceAccounts[0]?.id || "";
  const selectedAuditServiceAccount =
    serviceAccounts.find((item) => item.id === selectedAuditServiceAccountIDValue) ?? serviceAccounts[0] ?? null;
  const pendingInvitations = invitations.filter((item) => item.status === "pending");
  const latestInviteURL = createInvitationMutation.data?.accept_url ?? "";
  const currentUserID = session?.subject_id ?? "";
  const activeAdminCount = members.filter((member) => member.status === "active" && member.role === "admin").length;
  const activeMembers = members.filter((member) => member.status === "active");
  const disabledMembers = members.filter((member) => member.status !== "active");
  const disabledServiceAccounts = serviceAccounts.filter((account) => account.status === "disabled");
  const activeCredentialCount = serviceAccounts.reduce((sum, account) => sum + account.active_credential_count, 0);
  const selectedServiceAccountCredentials = selectedServiceAccount?.credentials ?? [];

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
  const auditItems = serviceAccountAuditQuery.data?.items ?? [];
  const selectedAuditEventIDValue =
    selectedAuditEventID && auditItems.some((item) => item.id === selectedAuditEventID) ? selectedAuditEventID : "";
  const selectedAuditEvent = auditItems.find((item) => item.id === selectedAuditEventIDValue) ?? null;
  const scrollToSection = (ref: { current: HTMLElement | null }) => {
    window.requestAnimationFrame(() => {
      ref.current?.scrollIntoView({ behavior: "smooth", block: "start" });
    });
  };
  const openAudit = (serviceAccountID: string) => {
    setSelectedAuditServiceAccountID(serviceAccountID);
    scrollToSection(auditSectionRef);
  };

  const downloadAuditCSV = (serviceAccountID: string) => {
    const path = `/v1/workspace/service-accounts/${encodeURIComponent(serviceAccountID)}/audit`;
    const url = new URL(path, apiBaseURL || window.location.origin);
    url.searchParams.set("limit", "500");
    url.searchParams.set("format", "csv");

    const link = document.createElement("a");
    link.href = url.toString();
    link.download = `service-account-${serviceAccountID}-audit.csv`;
    document.body.appendChild(link);
    link.click();
    link.remove();
  };

  const isSelfMember = (userID: string): boolean => currentUserID !== "" && currentUserID === userID;
  const isLastActiveAdmin = (member: { role: string; status: string }): boolean =>
    member.status === "active" && member.role === "admin" && activeAdminCount <= 1;

  const pagedMembers = paginateItems(members, memberPage, 6);
  const pagedInvitations = paginateItems(pendingInvitations, invitePage, 5);
  const pagedServiceAccounts = paginateItems(serviceAccounts, serviceAccountPage, 5);
  const pagedCredentials = paginateItems(selectedServiceAccountCredentials, credentialPage, 4);

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
            <section className="rounded-[28px] border border-slate-200 bg-[linear-gradient(135deg,#0f172a_0%,#1e293b_42%,#e2e8f0_42%,#f8fafc_100%)] p-[1px] shadow-sm">
              <div className="rounded-[27px] bg-white">
                <div className="grid gap-5 rounded-[27px] bg-[radial-gradient(circle_at_top_left,rgba(15,23,42,0.06),transparent_34%),linear-gradient(180deg,#ffffff,rgba(248,250,252,0.98))] p-6 lg:grid-cols-[1.15fr_0.85fr]">
                  <div>
                    <p className="text-xs uppercase tracking-[0.24em] text-slate-500">Workspace access governance</p>
                    <h1 className="mt-3 text-3xl font-semibold tracking-tight text-slate-950">Identity, credentials, and audit evidence in one operating console</h1>
                    <p className="mt-3 max-w-2xl text-sm leading-7 text-slate-600">
                      Keep human access, machine credentials, and credential evidence under one review path. Use people access for operators, service accounts for automation, and audit events for evidence.
                    </p>
                    <div className="mt-5 flex flex-wrap gap-2">
                      <button
                        type="button"
                        onClick={() => scrollToSection(peopleSectionRef)}
                        className="inline-flex h-10 items-center rounded-xl border border-slate-900 bg-slate-900 px-4 text-xs font-semibold uppercase tracking-[0.14em] text-white transition hover:bg-slate-800"
                      >
                        People access
                      </button>
                      <button
                        type="button"
                        onClick={() => scrollToSection(machineSectionRef)}
                        className="inline-flex h-10 items-center rounded-xl border border-stone-200 bg-white px-4 text-xs font-semibold uppercase tracking-[0.14em] text-slate-700 transition hover:bg-stone-100"
                      >
                        Machine access
                      </button>
                      <button
                        type="button"
                        onClick={() => scrollToSection(auditSectionRef)}
                        className="inline-flex h-10 items-center rounded-xl border border-stone-200 bg-white px-4 text-xs font-semibold uppercase tracking-[0.14em] text-slate-700 transition hover:bg-stone-100"
                      >
                        Audit evidence
                      </button>
                    </div>
                  </div>
                  <div className="grid gap-3 rounded-3xl border border-slate-200 bg-white/80 p-5 shadow-sm">
                    <div>
                      <p className="text-[11px] font-semibold uppercase tracking-[0.18em] text-slate-500">Current posture</p>
                      <p className="mt-2 text-lg font-semibold text-slate-950">
                        {pendingInvitations.length > 0 || disabledServiceAccounts.length > 0 || disabledMembers.length > 0
                          ? "Attention required"
                          : "Controlled"}
                      </p>
                      <p className="mt-2 text-sm text-slate-600">
                        {pendingInvitations.length} pending invite{pendingInvitations.length === 1 ? "" : "s"} · {disabledServiceAccounts.length} disabled machine identit{disabledServiceAccounts.length === 1 ? "y" : "ies"} · {disabledMembers.length} inactive member{disabledMembers.length === 1 ? "" : "s"}
                      </p>
                    </div>
                    <div className="grid gap-3 sm:grid-cols-2">
                      <DetailField label="Active members" value={`${activeMembers.length}`} className="bg-white" />
                      <DetailField label="Credential inventory" value={`${activeCredentialCount} active`} className="bg-white" />
                    </div>
                  </div>
                </div>
              </div>
            </section>

            <section className="grid gap-4 md:grid-cols-4">
              <SummaryMetric label="Members" value={String(members.length)} hint={`${activeMembers.length} active operators`} />
              <SummaryMetric label="Pending invites" value={String(pendingInvitations.length)} hint="People waiting for workspace entry" />
              <SummaryMetric label="Service accounts" value={String(serviceAccounts.length)} hint={`${disabledServiceAccounts.length} disabled identities`} />
              <SummaryMetric label="Active credentials" value={String(activeCredentialCount)} hint="Machine secrets currently usable" />
            </section>

            <section className="grid gap-3 xl:grid-cols-3">
              <OperatorGuidanceCard title="Human access" body="Keep role changes, suspensions, and invitations in one review lane. Avoid mixing person access with credential inventory." />
              <OperatorGuidanceCard title="Machine access" body="Each service account should map to one automation job, clear purpose, and explicit environment." />
              <OperatorGuidanceCard title="Evidence path" body="Review the event trail before exporting CSV. Evidence should follow the service account you are changing." />
            </section>

            <section ref={peopleSectionRef} className="rounded-3xl border border-stone-200 bg-white p-6 shadow-sm">
              <div className="flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between">
                <div>
                  <p className="text-xs uppercase tracking-[0.2em] text-slate-500">Human access</p>
                  <h2 className="mt-2 text-2xl font-semibold tracking-tight text-slate-950">Membership and invitation control</h2>
                  <p className="mt-2 max-w-2xl text-sm leading-7 text-slate-600">
                    Invite operators, adjust effective role, and suspend stale membership from one review surface.
                  </p>
                </div>
                <div className="grid min-w-[260px] gap-3 sm:grid-cols-2">
                  <DetailField label="Active admins" value={`${activeAdminCount}`} className="bg-white" />
                  <DetailField label="Pending onboarding" value={`${pendingInvitations.length}`} className="bg-white" />
                </div>
              </div>

              <div className="mt-6 grid gap-4 xl:grid-cols-[0.9fr_1.1fr]">
                <div className="rounded-2xl border border-stone-200 bg-stone-50 p-5">
                  <p className="text-xs uppercase tracking-[0.16em] text-slate-500">Invite operator</p>
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
                    <div className="mt-4 rounded-2xl border border-stone-200 bg-white px-4 py-3">
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

                <div className="grid gap-4">
                  <div className="rounded-2xl border border-stone-200 bg-stone-50 p-5">
                    <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
                      <div>
                        <p className="text-xs uppercase tracking-[0.16em] text-slate-500">Pending invites</p>
                        <p className="mt-1 text-sm text-slate-600">Temporary access awaiting acceptance.</p>
                      </div>
                      <PaginationControls page={pagedInvitations.page} totalPages={pagedInvitations.totalPages} onPageChange={setInvitePage} label="Pending invites" />
                    </div>
                    <div className="mt-4 grid gap-3">
                      {pagedInvitations.items.length > 0 ? (
                        pagedInvitations.items.map((invite) => (
                          <div key={invite.id} className="rounded-2xl border border-stone-200 bg-white px-4 py-3">
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

                  <div className="rounded-2xl border border-stone-200 bg-stone-50 p-5">
                    <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
                      <div>
                        <p className="text-xs uppercase tracking-[0.16em] text-slate-500">Current members</p>
                        <p className="mt-1 text-sm text-slate-600">Review effective role and active operator footprint.</p>
                      </div>
                      <PaginationControls page={pagedMembers.page} totalPages={pagedMembers.totalPages} onPageChange={setMemberPage} label="Current members" />
                    </div>
                    <div className="mt-4 grid gap-3">
                      {pagedMembers.items.length > 0 ? (
                        pagedMembers.items.map((member) => {
                          const draftRole = memberDraftRoles[member.user_id] ?? (member.role as "reader" | "writer" | "admin");
                          const roleDirty = draftRole !== member.role;
                          const selfMember = isSelfMember(member.user_id);
                          const lastAdminProtected = isLastActiveAdmin(member);
                          const showSuspendConfirm =
                            confirmingMemberAction?.userID === member.user_id && confirmingMemberAction.action === "suspend";
                          const roleSelectDisabled =
                            member.status !== "active" || updateMemberMutation.isPending || selfMember || lastAdminProtected;
                          const canApplyRole = roleDirty && !roleSelectDisabled && Boolean(csrfToken);
                          const canSuspend =
                            member.status === "active" &&
                            Boolean(csrfToken) &&
                            !removeMemberMutation.isPending &&
                            !selfMember &&
                            !lastAdminProtected;
                          const canReactivate =
                            member.status !== "active" && Boolean(csrfToken) && !updateMemberMutation.isPending && !selfMember;

                          return (
                            <div key={member.user_id} className="rounded-2xl border border-stone-200 bg-white px-4 py-4">
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
                            </div>
                          );
                        })
                      ) : (
                        <p className="text-sm text-slate-500">No active members yet.</p>
                      )}
                    </div>
                  </div>
                </div>
              </div>
            </section>

            <section ref={machineSectionRef} className="rounded-3xl border border-stone-200 bg-white p-6 shadow-sm">
              <div className="flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between">
                <div>
                  <p className="text-xs uppercase tracking-[0.2em] text-slate-500">Machine access</p>
                  <h2 className="mt-2 text-2xl font-semibold tracking-tight text-slate-950">Service-account inventory and credential posture</h2>
                  <p className="mt-2 max-w-2xl text-sm leading-7 text-slate-600">
                    Keep automation identities reviewable: clear owner, explicit purpose, controlled rotation, and visible disable state.
                  </p>
                </div>
                <div className="grid min-w-[260px] gap-3 sm:grid-cols-2">
                  <DetailField label="Service accounts" value={`${serviceAccounts.length}`} className="bg-white" />
                  <DetailField label="Disabled identities" value={`${disabledServiceAccounts.length}`} className="bg-white" />
                </div>
              </div>

              <div className="mt-6 grid gap-4 xl:grid-cols-[0.95fr_1.05fr]">
                <div className="grid gap-4">
                  <div className="rounded-2xl border border-stone-200 bg-stone-50 p-5">
                    <p className="text-xs uppercase tracking-[0.16em] text-slate-500">Create service account</p>
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
                        <p className="text-[11px] uppercase tracking-[0.14em] text-emerald-700">Latest issued secret</p>
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

                  <div className="rounded-2xl border border-stone-200 bg-stone-50 p-5">
                    <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
                      <div>
                        <p className="text-xs uppercase tracking-[0.16em] text-slate-500">Service-account inventory</p>
                        <p className="mt-1 text-sm text-slate-600">Select an automation identity to inspect its credential posture.</p>
                      </div>
                      <PaginationControls page={pagedServiceAccounts.page} totalPages={pagedServiceAccounts.totalPages} onPageChange={setServiceAccountPage} label="Service accounts" />
                    </div>
                    <div className="mt-4 grid gap-3">
                      {pagedServiceAccounts.items.length > 0 ? (
                        pagedServiceAccounts.items.map((account) => (
                          <button
                            key={account.id}
                            type="button"
                            onClick={() => setSelectedServiceAccountID(account.id)}
                            aria-pressed={selectedServiceAccountIDValue === account.id}
                            className={`rounded-2xl border px-4 py-4 text-left transition ${
                              selectedServiceAccountIDValue === account.id
                                ? "border-emerald-300 bg-emerald-50/60 shadow-sm"
                                : "border-stone-200 bg-white hover:border-stone-300 hover:bg-stone-50"
                            }`}
                          >
                            <div className="flex items-start justify-between gap-3">
                              <div className="min-w-0">
                                <div className="flex flex-wrap items-center gap-2">
                                  <p className="text-sm font-medium text-slate-950">{account.name}</p>
                                  <span
                                    className={`rounded-full px-2.5 py-1 text-[11px] font-semibold uppercase tracking-[0.14em] ${
                                      account.status === "active"
                                        ? "border border-emerald-200 bg-emerald-50 text-emerald-700"
                                        : "border border-stone-200 bg-white text-slate-500"
                                    }`}
                                  >
                                    {formatServiceAccountStatus(account.status)}
                                  </span>
                                </div>
                                <p className="mt-1 text-xs text-slate-500">
                                  {formatServiceAccountRole(account.role)} · {(account.environment || "unspecified").toUpperCase()}
                                </p>
                                <p className="mt-2 text-sm text-slate-700">{account.description || "No description recorded."}</p>
                                <p className="mt-2 text-xs text-slate-500">
                                  {account.active_credential_count} active credential{account.active_credential_count === 1 ? "" : "s"} · {describeServiceAccountActivity(account)}
                                </p>
                              </div>
                              <button
                                type="button"
                                onClick={(event) => {
                                  event.stopPropagation();
                                  openAudit(account.id);
                                }}
                                className="inline-flex h-9 items-center gap-2 rounded-xl border border-stone-200 bg-white px-3 text-xs text-slate-700 transition hover:bg-stone-100"
                              >
                                Audit
                              </button>
                            </div>
                          </button>
                        ))
                      ) : (
                        <div className="rounded-2xl border border-dashed border-stone-300 bg-white px-4 py-6 text-sm text-slate-600">
                          No service accounts yet. Create one to issue a machine credential and track its audit history.
                        </div>
                      )}
                    </div>
                  </div>
                </div>

                {selectedServiceAccount ? (
                  <div className="rounded-2xl border border-stone-200 bg-stone-50 px-4 py-4">
                    <div className="flex flex-col gap-3 lg:flex-row lg:items-start lg:justify-between">
                      <div>
                        <p className="text-xs uppercase tracking-[0.14em] text-slate-500">Selected service account</p>
                        <p className="mt-2 text-lg font-semibold text-slate-950">{selectedServiceAccount.name}</p>
                        <p className="mt-1 text-sm text-slate-700">{selectedServiceAccount.description || "No description recorded."}</p>
                        <p className="mt-2 text-xs text-slate-500">
                          Purpose: {selectedServiceAccount.purpose || "Not recorded"} · Environment: {(selectedServiceAccount.environment || "unspecified").toUpperCase()}
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
                          onClick={() => openAudit(selectedServiceAccount.id)}
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
                      <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
                        <div>
                          <p className="text-xs uppercase tracking-[0.14em] text-slate-500">Current credentials</p>
                          <p className="mt-1 text-sm text-slate-600">Review issue, rotation, and revocation state.</p>
                        </div>
                        <PaginationControls page={pagedCredentials.page} totalPages={pagedCredentials.totalPages} onPageChange={setCredentialPage} label="Credentials" />
                      </div>
                      <div className="mt-3 grid gap-3">
                        {pagedCredentials.items.length > 0 ? (
                          pagedCredentials.items.map((credential) => {
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

            <section ref={auditSectionRef} className="rounded-3xl border border-stone-200 bg-white p-6 shadow-sm">
              <div className="flex flex-col gap-3 lg:flex-row lg:items-start lg:justify-between">
                <div>
                  <p className="text-xs uppercase tracking-[0.2em] text-slate-500">Audit evidence</p>
                  <h2 className="mt-2 text-2xl font-semibold tracking-tight text-slate-950">Credential event timeline and export record</h2>
                  <p className="mt-2 max-w-2xl text-sm leading-7 text-slate-600">Review credential lifecycle events and export evidence only when operators need a durable record.</p>
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
                    onClick={() => downloadAuditCSV(selectedAuditServiceAccountIDValue)}
                    disabled={!selectedAuditServiceAccountIDValue}
                    className="inline-flex h-10 items-center gap-2 rounded-xl border border-stone-200 bg-white px-3 text-xs uppercase tracking-[0.12em] text-slate-700 transition hover:bg-stone-100 disabled:cursor-not-allowed disabled:opacity-50"
                  >
                    <Download className="h-3.5 w-3.5" />
                    Download audit CSV
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
                      {auditItems.length > 0 ? (
                        auditItems.map((event) => (
                          <ServiceAccountAuditRow
                            key={event.id}
                            event={event}
                            selected={event.id === selectedAuditEventIDValue}
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
                      <p className="text-xs uppercase tracking-[0.14em] text-slate-500">Scheduled exports</p>
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

function SummaryMetric({ label, value, hint }: { label: string; value: string; hint?: string }) {
  return (
    <div className="rounded-2xl border border-stone-200 bg-white px-4 py-4 shadow-sm">
      <p className="text-[11px] font-semibold uppercase tracking-[0.15em] text-slate-500">{label}</p>
      <p className="mt-2 text-base font-semibold text-slate-950">{value}</p>
      {hint ? <p className="mt-1 text-xs text-slate-500">{hint}</p> : null}
    </div>
  );
}

function OperatorGuidanceCard({ title, body }: { title: string; body: string }) {
  return (
    <section className="rounded-2xl border border-stone-200 bg-white p-5 shadow-sm">
      <p className="text-sm font-semibold text-slate-950">{title}</p>
      <p className="mt-2 text-sm leading-relaxed text-slate-600">{body}</p>
    </section>
  );
}

function ServiceAccountAuditDetail({ event }: { event: APIKeyAuditEvent | null }) {
  if (!event) {
    return (
      <div className="rounded-2xl border border-dashed border-stone-300 bg-white px-4 py-6 text-sm text-slate-600">
        Select an audit event to inspect the full credential record.
      </div>
    );
  }

  const metadata = event.metadata ?? {};
  const presentation = describeAuditEvent(event);
  const createdByUserID = readAuditMetadataString(metadata, "created_by_user_id");
  const environment = readAuditMetadataString(metadata, "environment");
  const credentialName = readAuditMetadataString(metadata, "name");
  const ownerID = readAuditMetadataString(metadata, "owner_id");
  const ownerType = readAuditMetadataString(metadata, "owner_type");
  const purpose = readAuditMetadataString(metadata, "purpose");
  const role = readAuditMetadataString(metadata, "role");
  const actorLabel = event.actor_api_key_id ? "Credential session" : "Admin session";
  const actorValue = event.actor_api_key_id || createdByUserID || "Workspace operator";
  const groupedKeys = new Set(["created_by_user_id", "environment", "name", "owner_id", "owner_type", "purpose", "role"]);
  const remainingEntries = Object.entries(metadata)
    .filter(([key]) => !groupedKeys.has(key))
    .sort(([left], [right]) => left.localeCompare(right));

  return (
    <div className="rounded-2xl border border-stone-200 bg-white px-4 py-4">
      <p className="text-xs uppercase tracking-[0.14em] text-slate-500">Selected event</p>
      <p className="mt-2 text-lg font-semibold text-slate-950">{presentation.title}</p>
      <p className="mt-2 text-sm text-slate-700">{presentation.summary}</p>
      <div className="mt-4 grid gap-3 sm:grid-cols-2">
        <DetailField label="What happened" value={presentation.title} />
        <DetailField label="Created at" value={formatExactTimestamp(event.created_at)} />
        <DetailField label="Who did it" value={actorLabel} />
        <DetailField label="Actor reference" value={actorValue} mono={Boolean(event.actor_api_key_id || createdByUserID)} />
      </div>

      <div className="mt-4 rounded-xl border border-stone-200 bg-stone-50 px-4 py-4">
        <p className="text-[11px] font-semibold uppercase tracking-[0.14em] text-slate-500">Credential record</p>
        <div className="mt-3 grid gap-3 sm:grid-cols-2">
          <DetailField label="Credential name" value={credentialName || "No display name recorded"} />
          <DetailField label="Access role" value={role ? formatServiceAccountRole(role as "reader" | "writer" | "admin") : "Not recorded"} />
          <DetailField label="Intended use" value={purpose || "Not recorded"} />
          <DetailField label="Environment" value={environment ? environment.toUpperCase() : "Not recorded"} />
        </div>
      </div>

      <div className="mt-4 rounded-xl border border-stone-200 bg-stone-50 px-4 py-4">
        <p className="text-[11px] font-semibold uppercase tracking-[0.14em] text-slate-500">Internal references</p>
        <div className="mt-3 grid gap-3 sm:grid-cols-2">
          <DetailField label="Event ID" value={event.id} mono />
          <DetailField label="Credential ID" value={event.api_key_id} mono />
          <DetailField label="Owner type" value={ownerType || "Not recorded"} />
          <DetailField label="Owner ID" value={ownerID || "Not recorded"} mono={Boolean(ownerID)} />
          {createdByUserID ? <DetailField label="Created by user ID" value={createdByUserID} mono /> : null}
          {event.actor_api_key_id ? <DetailField label="Actor credential ID" value={event.actor_api_key_id} mono /> : null}
        </div>
      </div>

      {remainingEntries.length > 0 ? (
        <div className="mt-4 rounded-xl border border-stone-200 bg-stone-50 px-4 py-4">
          <p className="text-[11px] font-semibold uppercase tracking-[0.14em] text-slate-500">Additional metadata</p>
          <dl className="mt-3 grid gap-3 sm:grid-cols-2">
            {remainingEntries.map(([key, value]) => (
              <div key={key} className="rounded-lg border border-stone-200 bg-white px-3 py-3">
                <dt className="text-[11px] font-semibold uppercase tracking-[0.14em] text-slate-500">{formatAuditMetadataKey(key)}</dt>
                <dd className="mt-2 break-words text-sm text-slate-800">{formatAuditMetadataValue(value)}</dd>
              </div>
            ))}
          </dl>
        </div>
      ) : null}
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
    <div className="inline-flex items-center gap-2 rounded-xl border border-stone-200 bg-white px-2 py-2">
      <button
        type="button"
        onClick={() => onPageChange(page - 1)}
        disabled={page <= 1}
        aria-label={`Previous ${label} page`}
        className="inline-flex h-8 w-8 items-center justify-center rounded-lg border border-stone-200 text-slate-700 transition hover:bg-stone-100 disabled:cursor-not-allowed disabled:opacity-50"
      >
        <ChevronLeft className="h-4 w-4" />
      </button>
      <span className="min-w-[84px] text-center text-[11px] font-semibold uppercase tracking-[0.14em] text-slate-500">
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

function readAuditMetadataString(metadata: Record<string, unknown>, key: string): string {
  const value = metadata[key];
  return typeof value === "string" ? value.trim() : "";
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
  const metadataSummary = metadataCount > 0 ? `${metadataCount} supporting field${metadataCount === 1 ? "" : "s"}` : "No supporting fields";

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
