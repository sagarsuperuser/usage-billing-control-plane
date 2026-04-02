"use client";

import { type ReactNode, useEffect, useRef, useState } from "react";
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
  const [selectedMemberID, setSelectedMemberID] = useState("");
  const [selectedServiceAccountID, setSelectedServiceAccountID] = useState("");
  const [selectedAuditServiceAccountID, setSelectedAuditServiceAccountID] = useState("");
  const [selectedAuditEventID, setSelectedAuditEventID] = useState("");
  const [memberDraftRoles, setMemberDraftRoles] = useState<Record<string, "reader" | "writer" | "admin">>({});
  const [confirmingMemberAction, setConfirmingMemberAction] = useState<{ userID: string; action: "suspend" } | null>(null);
  const [memberPage, setMemberPage] = useState(1);
  const [invitePage, setInvitePage] = useState(1);
  const [serviceAccountPage, setServiceAccountPage] = useState(1);
  const [credentialPage, setCredentialPage] = useState(1);
  const [auditPage, setAuditPage] = useState(1);
  const [auditCursor, setAuditCursor] = useState<string | undefined>(undefined);
  const [auditCursorHistory, setAuditCursorHistory] = useState<Array<string | undefined>>([]);
  const [auditExportPage, setAuditExportPage] = useState(1);
  const [auditExportCursor, setAuditExportCursor] = useState<string | undefined>(undefined);
  const [auditExportCursorHistory, setAuditExportCursorHistory] = useState<Array<string | undefined>>([]);

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
  const serviceAccountsWithoutActiveCredentials = serviceAccounts.filter((account) => account.active_credential_count === 0).length;
  const selectedMemberIDValue = selectedMemberID || members[0]?.user_id || "";
  const selectedMember = members.find((member) => member.user_id === selectedMemberIDValue) ?? members[0] ?? null;
  const selectedServiceAccountCredentials = selectedServiceAccount?.credentials ?? [];

  useEffect(() => {
    setSelectedAuditEventID("");
    setAuditPage(1);
    setAuditCursor(undefined);
    setAuditCursorHistory([]);
    setAuditExportPage(1);
    setAuditExportCursor(undefined);
    setAuditExportCursorHistory([]);
  }, [selectedAuditServiceAccountIDValue]);

  const serviceAccountAuditQuery = useQuery({
    queryKey: ["tenant-workspace-service-account-audit", apiBaseURL, session?.tenant_id, selectedAuditServiceAccountIDValue],
    queryFn: () =>
      fetchTenantWorkspaceServiceAccountAudit({
        runtimeBaseURL: apiBaseURL,
        serviceAccountID: selectedAuditServiceAccountIDValue,
        limit: 8,
        cursor: auditCursor,
      }),
    enabled: isAuthenticated && scope === "tenant" && isAdmin && selectedAuditServiceAccountIDValue !== "",
  });
  const serviceAccountAuditExportsQuery = useQuery({
    queryKey: ["tenant-workspace-service-account-audit-exports", apiBaseURL, session?.tenant_id, selectedAuditServiceAccountIDValue],
    queryFn: () =>
      fetchTenantWorkspaceServiceAccountAuditExports({
        runtimeBaseURL: apiBaseURL,
        serviceAccountID: selectedAuditServiceAccountIDValue,
        limit: 4,
        cursor: auditExportCursor,
      }),
    enabled: isAuthenticated && scope === "tenant" && isAdmin && selectedAuditServiceAccountIDValue !== "",
  });
  const auditItems = serviceAccountAuditQuery.data?.items ?? [];
  const auditExportItems = serviceAccountAuditExportsQuery.data?.items ?? [];
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
  const auditHasPreviousPage = auditCursorHistory.length > 0;
  const auditHasNextPage = Boolean(serviceAccountAuditQuery.data?.next_cursor);
  const auditExportHasPreviousPage = auditExportCursorHistory.length > 0;
  const auditExportHasNextPage = Boolean(serviceAccountAuditExportsQuery.data?.next_cursor);

  const goToNextAuditPage = () => {
    const nextCursor = serviceAccountAuditQuery.data?.next_cursor;
    if (!nextCursor) {
      return;
    }
    setAuditCursorHistory((current) => [...current, auditCursor]);
    setAuditCursor(nextCursor);
    setAuditPage((current) => current + 1);
    setSelectedAuditEventID("");
  };

  const goToPreviousAuditPage = () => {
    if (auditCursorHistory.length === 0) {
      return;
    }
    const previousCursor = auditCursorHistory[auditCursorHistory.length - 1];
    setAuditCursorHistory((current) => current.slice(0, -1));
    setAuditCursor(previousCursor);
    setAuditPage((current) => Math.max(1, current - 1));
    setSelectedAuditEventID("");
  };

  const goToNextAuditExportPage = () => {
    const nextCursor = serviceAccountAuditExportsQuery.data?.next_cursor;
    if (!nextCursor) {
      return;
    }
    setAuditExportCursorHistory((current) => [...current, auditExportCursor]);
    setAuditExportCursor(nextCursor);
    setAuditExportPage((current) => current + 1);
  };

  const goToPreviousAuditExportPage = () => {
    if (auditExportCursorHistory.length === 0) {
      return;
    }
    const previousCursor = auditExportCursorHistory[auditExportCursorHistory.length - 1];
    setAuditExportCursorHistory((current) => current.slice(0, -1));
    setAuditExportCursor(previousCursor);
    setAuditExportPage((current) => Math.max(1, current - 1));
  };

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
            <section className="flex flex-col gap-4 lg:flex-row lg:items-end lg:justify-between">
              <div>
                <p className="text-xs uppercase tracking-[0.2em] text-slate-500">Workspace access</p>
                <h1 className="mt-2 text-3xl font-semibold tracking-tight text-slate-950">People, machine identities, and audit controls</h1>
                <p className="mt-2 max-w-2xl text-sm leading-7 text-slate-600">
                  Manage human access, service accounts, and credential evidence from one workspace console.
                </p>
              </div>
              <div className="flex flex-wrap gap-2">
                <button
                  type="button"
                  onClick={() => scrollToSection(peopleSectionRef)}
                  className="inline-flex h-10 items-center rounded-xl border border-slate-900 bg-slate-900 px-4 text-xs font-semibold uppercase tracking-[0.14em] text-white transition hover:bg-slate-800"
                >
                  People
                </button>
                <button
                  type="button"
                  onClick={() => scrollToSection(machineSectionRef)}
                  className="inline-flex h-10 items-center rounded-xl border border-stone-200 bg-white px-4 text-xs font-semibold uppercase tracking-[0.14em] text-slate-700 transition hover:bg-stone-100"
                >
                  Machine
                </button>
                <button
                  type="button"
                  onClick={() => scrollToSection(auditSectionRef)}
                  className="inline-flex h-10 items-center rounded-xl border border-stone-200 bg-white px-4 text-xs font-semibold uppercase tracking-[0.14em] text-slate-700 transition hover:bg-stone-100"
                >
                  Audit
                </button>
              </div>
            </section>

            <section className="grid gap-4 md:grid-cols-4">
              <SummaryMetric label="Members" value={String(members.length)} hint={`${activeMembers.length} active operators`} />
              <SummaryMetric label="Pending invites" value={String(pendingInvitations.length)} hint="People waiting for workspace entry" />
              <SummaryMetric label="Service accounts" value={String(serviceAccounts.length)} hint={`${disabledServiceAccounts.length} disabled identities`} />
              <SummaryMetric label="Active credentials" value={String(activeCredentialCount)} hint="Machine secrets currently usable" />
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
                  <DetailField label="Inactive members" value={`${disabledMembers.length}`} className="bg-white" />
                </div>
              </div>

              <div className="mt-6 grid gap-4 xl:grid-cols-[1.12fr_0.88fr]">
                <div className="grid gap-4">
                  <section className="overflow-hidden rounded-2xl border border-stone-200 bg-white">
                    <div className="flex flex-col gap-3 border-b border-stone-200 bg-stone-50 px-5 py-4 sm:flex-row sm:items-center sm:justify-between">
                      <div>
                        <p className="text-xs uppercase tracking-[0.16em] text-slate-500">Current members</p>
                        <p className="mt-1 text-sm text-slate-600">Workspace operator inventory with direct role and status review.</p>
                      </div>
                      <PaginationControls page={pagedMembers.page} totalPages={pagedMembers.totalPages} onPageChange={setMemberPage} label="Current members" />
                    </div>
                    {pagedMembers.items.length > 0 ? (
                      <div className="divide-y divide-stone-200">
                        <div className="hidden grid-cols-[minmax(0,2.2fr)_120px_140px_140px] gap-4 bg-stone-50 px-5 py-3 text-[11px] font-semibold uppercase tracking-[0.14em] text-slate-500 md:grid">
                          <span>Operator</span>
                          <span>Role</span>
                          <span>Status</span>
                          <span>Control</span>
                        </div>
                        {pagedMembers.items.map((member) => {
                          const selected = selectedMemberIDValue === member.user_id;
                          return (
                            <button
                              key={member.user_id}
                              type="button"
                              onClick={() => setSelectedMemberID(member.user_id)}
                              className={`grid w-full gap-3 px-5 py-4 text-left transition md:grid-cols-[minmax(0,2.2fr)_120px_140px_140px] md:items-center ${
                                selected ? "bg-sky-50/70" : "bg-white hover:bg-stone-50"
                              }`}
                            >
                              <div className="min-w-0">
                                <p className="truncate text-sm font-medium text-slate-950">{member.display_name}</p>
                                <p className="mt-1 truncate text-xs text-slate-500">{member.email}</p>
                              </div>
                              <span className="text-sm text-slate-700">{member.role}</span>
                              <div className="flex flex-wrap gap-2">
                                <StatusChip tone={member.status === "active" ? "success" : "neutral"}>{member.status}</StatusChip>
                                {isSelfMember(member.user_id) ? <StatusChip tone="info">You</StatusChip> : null}
                                {isLastActiveAdmin(member) ? <StatusChip tone="warning">Last admin</StatusChip> : null}
                              </div>
                              <span className="text-xs font-semibold uppercase tracking-[0.12em] text-slate-500">
                                {selected ? "Selected" : "Inspect"}
                              </span>
                            </button>
                          );
                        })}
                      </div>
                    ) : (
                      <div className="px-5 py-8 text-sm text-slate-500">No active members yet.</div>
                    )}
                  </section>

                  <section className="overflow-hidden rounded-2xl border border-stone-200 bg-white">
                    <div className="flex flex-col gap-3 border-b border-stone-200 bg-stone-50 px-5 py-4 sm:flex-row sm:items-center sm:justify-between">
                      <div>
                        <p className="text-xs uppercase tracking-[0.16em] text-slate-500">Pending invites</p>
                        <p className="mt-1 text-sm text-slate-600">Temporary access links waiting for acceptance.</p>
                      </div>
                      <PaginationControls page={pagedInvitations.page} totalPages={pagedInvitations.totalPages} onPageChange={setInvitePage} label="Pending invites" />
                    </div>
                    {pagedInvitations.items.length > 0 ? (
                      <div className="divide-y divide-stone-200">
                        <div className="hidden grid-cols-[minmax(0,1.8fr)_120px_180px_110px] gap-4 bg-stone-50 px-5 py-3 text-[11px] font-semibold uppercase tracking-[0.14em] text-slate-500 md:grid">
                          <span>Email</span>
                          <span>Role</span>
                          <span>Expires</span>
                          <span>Action</span>
                        </div>
                        {pagedInvitations.items.map((invite) => (
                          <div key={invite.id} className="grid gap-3 px-5 py-4 md:grid-cols-[minmax(0,1.8fr)_120px_180px_110px] md:items-center">
                            <div className="min-w-0">
                              <p className="truncate text-sm font-medium text-slate-950">{invite.email}</p>
                            </div>
                            <span className="text-sm text-slate-700">{invite.role}</span>
                            <span className="text-sm text-slate-600">{formatExactTimestamp(invite.expires_at)}</span>
                            <button
                              type="button"
                              onClick={() => revokeInvitationMutation.mutate(invite.id)}
                              disabled={!csrfToken || revokeInvitationMutation.isPending}
                              className="inline-flex h-9 items-center justify-center rounded-lg border border-stone-200 bg-white px-3 text-xs font-medium text-slate-700 transition hover:bg-stone-100 disabled:cursor-not-allowed disabled:opacity-50"
                            >
                              Revoke
                            </button>
                          </div>
                        ))}
                      </div>
                    ) : (
                      <div className="px-5 py-8 text-sm text-slate-500">No pending workspace invites.</div>
                    )}
                  </section>
                </div>

                <div className="grid gap-4 xl:sticky xl:top-6 xl:self-start">
                  <section className="rounded-2xl border border-stone-200 bg-white p-5">
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
                  </section>

                  {selectedMember ? (
                    <section className="rounded-2xl border border-stone-200 bg-white p-5">
                      {(() => {
                        const draftRole = memberDraftRoles[selectedMember.user_id] ?? (selectedMember.role as "reader" | "writer" | "admin");
                        const roleDirty = draftRole !== selectedMember.role;
                        const selfMember = isSelfMember(selectedMember.user_id);
                        const lastAdminProtected = isLastActiveAdmin(selectedMember);
                        const showSuspendConfirm =
                          confirmingMemberAction?.userID === selectedMember.user_id && confirmingMemberAction.action === "suspend";
                        const roleSelectDisabled =
                          selectedMember.status !== "active" || updateMemberMutation.isPending || selfMember || lastAdminProtected;
                        const canApplyRole = roleDirty && !roleSelectDisabled && Boolean(csrfToken);
                        const canSuspend =
                          selectedMember.status === "active" &&
                          Boolean(csrfToken) &&
                          !removeMemberMutation.isPending &&
                          !selfMember &&
                          !lastAdminProtected;
                        const canReactivate =
                          selectedMember.status !== "active" && Boolean(csrfToken) && !updateMemberMutation.isPending && !selfMember;

                        return (
                          <>
                            <div className="flex items-start justify-between gap-3">
                              <div className="min-w-0">
                                <p className="text-xs uppercase tracking-[0.16em] text-slate-500">Selected member</p>
                                <h3 className="mt-2 truncate text-lg font-semibold text-slate-950">{selectedMember.display_name}</h3>
                                <p className="mt-1 break-all text-sm text-slate-600">{selectedMember.email}</p>
                              </div>
                              <StatusChip tone={selectedMember.status === "active" ? "success" : "neutral"}>{selectedMember.status}</StatusChip>
                            </div>
                            <div className="mt-4 grid gap-3 sm:grid-cols-2">
                              <DetailField label="Current role" value={selectedMember.role} />
                              <DetailField label="Operator flags" value={selfMember ? "Self" : lastAdminProtected ? "Last active admin" : "Standard"} />
                            </div>
                            <div className="mt-4 grid gap-3">
                              <select
                                aria-label={`Role for ${selectedMember.email}`}
                                value={draftRole}
                                onChange={(event) =>
                                  setMemberDraftRoles((current) => ({
                                    ...current,
                                    [selectedMember.user_id]: event.target.value as "reader" | "writer" | "admin",
                                  }))
                                }
                                disabled={roleSelectDisabled}
                                className="h-11 rounded-xl border border-stone-200 bg-white px-3 text-sm text-slate-900 outline-none ring-slate-400 transition focus:ring-2"
                              >
                                <option value="admin">Admin</option>
                                <option value="writer">Writer</option>
                                <option value="reader">Reader</option>
                              </select>
                              {roleDirty ? (
                                <div className="flex flex-wrap gap-2">
                                  <button
                                    type="button"
                                    onClick={() => updateMemberMutation.mutate({ userID: selectedMember.user_id, role: draftRole })}
                                    disabled={!canApplyRole}
                                    className="inline-flex h-10 items-center justify-center rounded-xl border border-slate-900 bg-slate-900 px-4 text-xs font-semibold uppercase tracking-[0.12em] text-white transition hover:bg-slate-800 disabled:cursor-not-allowed disabled:opacity-50"
                                  >
                                    Apply role
                                  </button>
                                  <button
                                    type="button"
                                    onClick={() =>
                                      setMemberDraftRoles((current) => {
                                        const next = { ...current };
                                        delete next[selectedMember.user_id];
                                        return next;
                                      })
                                    }
                                    className="inline-flex h-10 items-center justify-center rounded-xl border border-stone-200 bg-white px-4 text-xs font-semibold uppercase tracking-[0.12em] text-slate-700 transition hover:bg-stone-100"
                                  >
                                    Cancel
                                  </button>
                                </div>
                              ) : selectedMember.status === "active" ? (
                                showSuspendConfirm ? (
                                  <div className="flex flex-wrap gap-2">
                                    <button
                                      type="button"
                                      onClick={() => removeMemberMutation.mutate(selectedMember.user_id)}
                                      disabled={!canSuspend}
                                      className="inline-flex h-10 items-center justify-center rounded-xl border border-rose-700 bg-rose-700 px-4 text-xs font-semibold uppercase tracking-[0.12em] text-white transition hover:bg-rose-800 disabled:cursor-not-allowed disabled:opacity-50"
                                    >
                                      Confirm suspend
                                    </button>
                                    <button
                                      type="button"
                                      onClick={() => setConfirmingMemberAction(null)}
                                      className="inline-flex h-10 items-center justify-center rounded-xl border border-stone-200 bg-white px-4 text-xs font-semibold uppercase tracking-[0.12em] text-slate-700 transition hover:bg-stone-100"
                                    >
                                      Cancel
                                    </button>
                                  </div>
                                ) : (
                                  <button
                                    type="button"
                                    onClick={() => setConfirmingMemberAction({ userID: selectedMember.user_id, action: "suspend" })}
                                    disabled={!canSuspend}
                                    className="inline-flex h-10 items-center justify-center rounded-xl border border-rose-200 bg-rose-50 px-4 text-xs font-semibold uppercase tracking-[0.12em] text-rose-700 transition hover:bg-rose-100 disabled:cursor-not-allowed disabled:opacity-50"
                                  >
                                    Suspend member
                                  </button>
                                )
                              ) : (
                                <button
                                  type="button"
                                  onClick={() => updateMemberMutation.mutate({ userID: selectedMember.user_id, role: selectedMember.role as "reader" | "writer" | "admin" })}
                                  disabled={!canReactivate}
                                  className="inline-flex h-10 items-center justify-center rounded-xl border border-emerald-200 bg-emerald-50 px-4 text-xs font-semibold uppercase tracking-[0.12em] text-emerald-700 transition hover:bg-emerald-100 disabled:cursor-not-allowed disabled:opacity-50"
                                >
                                  Reactivate member
                                </button>
                              )}
                              {selfMember ? <p className="text-xs text-slate-500">You cannot change your own membership from this screen.</p> : null}
                              {lastAdminProtected ? <p className="text-xs text-slate-500">Promote another active admin before changing this member&apos;s access.</p> : null}
                            </div>
                          </>
                        );
                      })()}
                    </section>
                  ) : null}
                </div>
              </div>
            </section>

            <section ref={machineSectionRef} className="rounded-3xl border border-stone-200 bg-white p-6 shadow-sm">
              <div className="flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between">
                <div>
                  <p className="text-xs uppercase tracking-[0.2em] text-slate-500">Machine access</p>
                  <h2 className="mt-2 text-2xl font-semibold tracking-tight text-slate-950">Machine identities</h2>
                  <p className="mt-2 max-w-2xl text-sm leading-7 text-slate-600">Review automation identities, then issue or rotate credentials from the selected identity.</p>
                </div>
                <div className="grid min-w-[260px] gap-3 sm:grid-cols-2">
                  <DetailField label="Disabled identities" value={`${disabledServiceAccounts.length}`} className="bg-white" />
                  <DetailField label="No active credential" value={`${serviceAccountsWithoutActiveCredentials}`} className="bg-white" />
                </div>
              </div>

              <div className="mt-6 grid gap-4 xl:grid-cols-[1.08fr_0.92fr]">
                <div className="overflow-hidden rounded-2xl border border-stone-200 bg-white">
                  <div className="flex flex-col gap-3 border-b border-stone-200 bg-stone-50 px-5 py-4 sm:flex-row sm:items-center sm:justify-between">
                    <div>
                      <p className="text-xs uppercase tracking-[0.16em] text-slate-500">Service accounts</p>
                      <p className="mt-1 text-sm text-slate-600">Select an identity to inspect controls, credentials, and audit history.</p>
                    </div>
                    <PaginationControls page={pagedServiceAccounts.page} totalPages={pagedServiceAccounts.totalPages} onPageChange={setServiceAccountPage} label="Service accounts" />
                  </div>
                  {pagedServiceAccounts.items.length > 0 ? (
                    <div className="divide-y divide-stone-200">
                      {pagedServiceAccounts.items.map((account) => {
                        const selected = selectedServiceAccountIDValue === account.id;
                        return (
                          <div
                            key={account.id}
                            className={`px-5 py-4 transition ${
                              selected ? "bg-sky-50/70" : "bg-white"
                            }`}
                          >
                            <div className="flex flex-col gap-4 xl:flex-row xl:items-center xl:justify-between">
                              <div className="min-w-0 flex-1">
                                <div className="flex flex-wrap items-center gap-2">
                                  <p className="truncate text-sm font-semibold text-slate-950">{account.name}</p>
                                  <StatusChip tone={account.status === "active" ? "success" : "neutral"}>{formatServiceAccountStatus(account.status)}</StatusChip>
                                  {selected ? <StatusChip tone="info">Selected</StatusChip> : null}
                                  {account.active_credential_count === 0 ? <StatusChip tone="warning">No active credential</StatusChip> : null}
                                </div>
                                <p className="mt-1 break-words text-sm text-slate-600">{account.description || "No description recorded."}</p>
                                <div className="mt-2 flex flex-wrap gap-x-4 gap-y-1 text-xs text-slate-500">
                                  <span>Role: {formatServiceAccountRole(account.role)}</span>
                                  <span>Environment: {(account.environment || "unspecified").toUpperCase()}</span>
                                  <span>Credentials: {account.active_credential_count}</span>
                                </div>
                              </div>
                              <div className="flex flex-wrap gap-2 xl:justify-end">
                                <button
                                  type="button"
                                  onClick={() => setSelectedServiceAccountID(account.id)}
                                  className={`inline-flex h-9 items-center justify-center rounded-lg px-3 text-xs font-medium transition ${
                                    selected
                                      ? "border border-sky-200 bg-sky-100 text-sky-800"
                                      : "border border-stone-200 bg-white text-slate-700 hover:bg-stone-100"
                                  }`}
                                >
                                  {selected ? "Viewing" : "Inspect"}
                                </button>
                                <button
                                  type="button"
                                  onClick={() => openAudit(account.id)}
                                  className="inline-flex h-9 items-center justify-center rounded-lg border border-stone-200 bg-white px-3 text-xs font-medium text-slate-700 transition hover:bg-stone-100"
                                >
                                  Audit
                                </button>
                              </div>
                            </div>
                          </div>
                        );
                      })}
                    </div>
                  ) : (
                    <div className="px-5 py-8 text-sm text-slate-500">No service accounts yet. Create one to issue a machine credential and track its audit history.</div>
                  )}
                </div>

                <div className="grid gap-4 xl:sticky xl:top-6 xl:self-start">
                  <section className="rounded-2xl border border-stone-200 bg-white p-5">
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
                  </section>

                  {selectedServiceAccount ? (
                  <section className="rounded-2xl border border-stone-200 bg-white p-5">
                    <div className="flex flex-col gap-4">
                      <div>
                        <p className="text-xs uppercase tracking-[0.14em] text-slate-500">Selected identity</p>
                        <p className="mt-2 text-lg font-semibold text-slate-950">{selectedServiceAccount.name}</p>
                        <p className="mt-1 break-words text-sm text-slate-600">{selectedServiceAccount.description || "No description recorded."}</p>
                        <div className="mt-3 flex flex-wrap gap-2">
                          <StatusChip tone={selectedServiceAccount.status === "active" ? "success" : "neutral"}>{formatServiceAccountStatus(selectedServiceAccount.status)}</StatusChip>
                          <StatusChip tone="neutral">{formatServiceAccountRole(selectedServiceAccount.role)}</StatusChip>
                          <StatusChip tone="neutral">{(selectedServiceAccount.environment || "unspecified").toUpperCase()}</StatusChip>
                          <StatusChip tone="info">
                            {selectedServiceAccount.active_credential_count} active credential{selectedServiceAccount.active_credential_count === 1 ? "" : "s"}
                          </StatusChip>
                        </div>
                      </div>

                      <div className="grid gap-3 sm:grid-cols-2">
                        <DetailField label="Purpose" value={selectedServiceAccount.purpose || "Not recorded"} />
                        <DetailField label="Last activity" value={describeServiceAccountActivity(selectedServiceAccount)} />
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
                          Audit
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

                    <div className="mt-5">
                      <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
                        <div>
                          <p className="text-xs uppercase tracking-[0.14em] text-slate-500">Current credentials</p>
                          <p className="mt-1 text-sm text-slate-600">Review issue, rotation, and revocation state.</p>
                        </div>
                        <PaginationControls page={pagedCredentials.page} totalPages={pagedCredentials.totalPages} onPageChange={setCredentialPage} label="Credentials" />
                      </div>
                      <div className="mt-3 overflow-hidden rounded-2xl border border-stone-200">
                        {pagedCredentials.items.length > 0 ? (
                          <>
                            <div className="hidden grid-cols-[minmax(0,1.5fr)_150px_160px_130px_170px] gap-4 bg-stone-50 px-4 py-3 text-[11px] font-semibold uppercase tracking-[0.14em] text-slate-500 md:grid">
                              <span>Credential</span>
                              <span>Prefix</span>
                              <span>Issued</span>
                              <span>Status</span>
                              <span>Controls</span>
                            </div>
                          {pagedCredentials.items.map((credential) => {
                            const isRevoked = Boolean(credential.revoked_at);
                            return (
                              <div key={credential.id} className="grid gap-3 border-t border-stone-200 px-4 py-4 md:grid-cols-[minmax(0,1.5fr)_150px_160px_130px_170px] md:items-center md:first:border-t-0">
                                <div className="min-w-0">
                                  <p className="truncate text-sm font-medium text-slate-950">{credential.name}</p>
                                  <p className="mt-1 text-xs text-slate-500">{describeCredentialActivity(credential)}</p>
                                </div>
                                <p className="font-mono text-sm text-slate-700">{credential.key_prefix}</p>
                                <p className="text-sm text-slate-600">{formatExactTimestamp(credential.created_at)}</p>
                                <div className="flex flex-wrap gap-2">
                                  <StatusChip tone={isRevoked ? "danger" : "success"}>{isRevoked ? "Revoked" : "Active"}</StatusChip>
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
                                    className="inline-flex h-9 items-center justify-center rounded-lg border border-stone-200 bg-white px-3 text-xs font-medium text-slate-700 transition hover:bg-stone-100 disabled:cursor-not-allowed disabled:opacity-50"
                                  >
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
                                    className="inline-flex h-9 items-center justify-center rounded-lg border border-rose-200 bg-rose-50 px-3 text-xs font-medium text-rose-700 transition hover:bg-rose-100 disabled:cursor-not-allowed disabled:opacity-50"
                                  >
                                    Revoke
                                  </button>
                                </div>
                              </div>
                            );
                          })}
                          </>
                        ) : (
                          <div className="px-4 py-6 text-sm text-slate-500">No credentials issued yet.</div>
                        )}
                      </div>
                    </div>
                  </section>
                ) : (
                  <div className="rounded-2xl border border-dashed border-stone-300 bg-stone-50 px-4 py-6 text-sm text-slate-600">
                    Create a service account to review credential posture and machine access.
                  </div>
                )}
                </div>
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
                <div className="mt-4 grid gap-3 rounded-2xl border border-stone-200 bg-stone-50 px-4 py-4 sm:grid-cols-3">
                  <DetailField label="Audit focus" value={selectedAuditServiceAccount.name} className="bg-white" />
                  <DetailField
                    label="Credential posture"
                    value={`${selectedAuditServiceAccount.active_credential_count} active credential${selectedAuditServiceAccount.active_credential_count === 1 ? "" : "s"}`}
                    className="bg-white"
                  />
                  <DetailField
                    label="Last machine activity"
                    value={describeServiceAccountActivity(selectedAuditServiceAccount)}
                    className="bg-white"
                  />
                </div>
              ) : (
                <p className="mt-4 text-sm text-slate-500">Create a service account first to inspect machine-credential audit.</p>
              )}
              {selectedAuditServiceAccount ? (
                <div className="mt-4 grid gap-4 xl:grid-cols-[1.1fr_0.9fr]">
                  <div className="overflow-hidden rounded-2xl border border-stone-200 bg-white">
                    <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
                      <div className="border-b border-stone-200 bg-stone-50 px-4 py-4 sm:flex-1 sm:border-b-0">
                        <p className="text-xs uppercase tracking-[0.14em] text-slate-500">Recent events</p>
                        <p className="mt-1 text-sm text-slate-600">Page through the credential event timeline.</p>
                      </div>
                      <div className="px-4 pb-4 sm:pb-0 sm:pr-4">
                        <CursorPaginationControls
                          page={auditPage}
                          hasPreviousPage={auditHasPreviousPage}
                          hasNextPage={auditHasNextPage}
                          onPrevious={goToPreviousAuditPage}
                          onNext={goToNextAuditPage}
                          label="Audit events"
                        />
                      </div>
                    </div>
                    <div className="divide-y divide-stone-200">
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
                        <p className="px-4 py-6 text-sm text-slate-500">No audit events yet.</p>
                      )}
                    </div>
                  </div>
                  <div className="grid gap-4">
                    <ServiceAccountAuditDetail event={selectedAuditEvent} />
                    <div className="overflow-hidden rounded-2xl border border-stone-200 bg-white">
                      <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
                        <div className="border-b border-stone-200 bg-stone-50 px-4 py-4 sm:flex-1 sm:border-b-0">
                          <p className="text-xs uppercase tracking-[0.14em] text-slate-500">Scheduled exports</p>
                          <p className="mt-1 text-sm text-slate-600">Review generated export jobs for this service account.</p>
                        </div>
                        <div className="px-4 pb-4 sm:pb-0 sm:pr-4">
                          <CursorPaginationControls
                            page={auditExportPage}
                            hasPreviousPage={auditExportHasPreviousPage}
                            hasNextPage={auditExportHasNextPage}
                            onPrevious={goToPreviousAuditExportPage}
                            onNext={goToNextAuditExportPage}
                            label="Audit exports"
                          />
                        </div>
                      </div>
                      <div className="divide-y divide-stone-200">
                        {auditExportItems.length > 0 ? (
                          <>
                            <div className="hidden grid-cols-[140px_150px_120px_120px] gap-4 bg-stone-50 px-4 py-3 text-[11px] font-semibold uppercase tracking-[0.14em] text-slate-500 md:grid">
                              <span>Status</span>
                              <span>Created</span>
                              <span>Rows</span>
                              <span>Output</span>
                            </div>
                            {auditExportItems.map((item) => (
                              <div key={item.job.id} className="grid gap-3 px-4 py-4 md:grid-cols-[140px_150px_120px_120px] md:items-center">
                                <div className="flex flex-wrap gap-2">
                                  <StatusChip tone={item.download_url ? "success" : item.job.status === "failed" ? "danger" : "info"}>
                                    {formatAuditExportStatus(item.job.status)}
                                  </StatusChip>
                                </div>
                                <p className="text-sm text-slate-700">{formatExactTimestamp(item.job.created_at)}</p>
                                <p className="text-sm text-slate-700">{item.job.row_count} row(s)</p>
                                {item.download_url ? (
                                  <a
                                    href={item.download_url}
                                    target="_blank"
                                    rel="noreferrer"
                                    className="inline-flex h-9 items-center justify-center gap-2 rounded-lg border border-stone-200 bg-white px-3 text-xs font-medium text-slate-700 transition hover:bg-stone-100"
                                  >
                                    <Download className="h-3.5 w-3.5" />
                                    Download
                                  </a>
                                ) : (
                                  <span className="text-xs text-slate-500">Pending</span>
                                )}
                              </div>
                            ))}
                          </>
                        ) : (
                          <p className="px-4 py-6 text-sm text-slate-500">No audit exports created yet.</p>
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
      className={`grid w-full gap-3 px-4 py-4 text-left transition md:grid-cols-[minmax(0,1.6fr)_150px_180px] md:items-center ${
        selected ? "bg-sky-50/70" : "bg-white hover:bg-stone-50"
      }`}
    >
      <div className="min-w-0">
        <p className="truncate text-sm font-medium text-slate-950">{presentation.title}</p>
        <p className="mt-1 text-sm text-slate-700">{presentation.summary}</p>
      </div>
      <p className="text-sm text-slate-600">{formatExactTimestamp(event.created_at)}</p>
      <div className="flex flex-wrap items-center gap-2 md:justify-end">
        <StatusChip tone={event.action === "revoked" ? "danger" : event.action === "rotated" ? "warning" : "success"}>
          {formatAuditActionLabel(event.action)}
        </StatusChip>
        <span className="text-xs text-slate-500">{presentation.supporting}</span>
      </div>
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
    <span className={`inline-flex h-7 items-center rounded-full border px-2.5 text-[11px] font-semibold uppercase tracking-[0.12em] ${toneClassName}`}>
      {children}
    </span>
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

function CursorPaginationControls({
  page,
  hasPreviousPage,
  hasNextPage,
  onPrevious,
  onNext,
  label,
}: {
  page: number;
  hasPreviousPage: boolean;
  hasNextPage: boolean;
  onPrevious: () => void;
  onNext: () => void;
  label: string;
}) {
  return (
    <div className="inline-flex items-center gap-2 rounded-xl border border-stone-200 bg-white px-2 py-2">
      <button
        type="button"
        onClick={onPrevious}
        disabled={!hasPreviousPage}
        aria-label={`Previous ${label} page`}
        className="inline-flex h-8 w-8 items-center justify-center rounded-lg border border-stone-200 text-slate-700 transition hover:bg-stone-100 disabled:cursor-not-allowed disabled:opacity-50"
      >
        <ChevronLeft className="h-4 w-4" />
      </button>
      <span className="min-w-[84px] text-center text-[11px] font-semibold uppercase tracking-[0.14em] text-slate-500">
        Page {page}
      </span>
      <button
        type="button"
        onClick={onNext}
        disabled={!hasNextPage}
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
