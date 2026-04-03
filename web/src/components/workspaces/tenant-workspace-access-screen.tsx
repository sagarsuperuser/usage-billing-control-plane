"use client";

import { type ReactNode, useEffect, useRef, useState } from "react";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import {
  ChevronLeft,
  ChevronRight,
  Copy,
  Download,
  KeyRound,
  LoaderCircle,
  MailPlus,
  Plus,
  RefreshCw,
  ServerCog,
  ShieldCheck,
  ShieldOff,
  UserRound,
  UserX,
  X,
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
  const { register: registerInvite, handleSubmit: handleInviteSubmit, reset: resetInvite } = useForm({
    resolver: zodResolver(z.object({ email: z.string().email(), role: z.enum(["reader", "writer", "admin"]) })),
    defaultValues: { email: "", role: "writer" as const },
  });
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
  const [activeTab, setActiveTab] = useState<"members" | "service-accounts" | "audit">("members");
  const [showNewSAModal, setShowNewSAModal] = useState(false);

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
    mutationFn: (data: { email: string; role: "reader" | "writer" | "admin" }) =>
      createTenantWorkspaceInvitation({
        runtimeBaseURL: apiBaseURL,
        csrfToken,
        email: data.email,
        role: data.role,
      }),
    onSuccess: async () => {
      resetInvite();
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

  /* eslint-disable react-hooks/set-state-in-effect */
  useEffect(() => {
    setSelectedAuditEventID("");
    setAuditPage(1);
    setAuditCursor(undefined);
    setAuditCursorHistory([]);
    setAuditExportPage(1);
    setAuditExportCursor(undefined);
    setAuditExportCursorHistory([]);
  }, [selectedAuditServiceAccountIDValue]);
  /* eslint-enable react-hooks/set-state-in-effect */

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
      <main className="mx-auto flex max-w-[1180px] flex-col gap-5 px-4 py-6 md:px-8 lg:px-10">
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
            body={`Only workspace admins can manage invitations, service accounts, roles, and member removal.`}
            actionHref="/customers"
            actionLabel="Open workspace home"
          />
        ) : null}

        {isAuthenticated && scope === "tenant" && isAdmin ? (
          <>
            <div>
              <h1 className="text-xl font-semibold tracking-tight text-slate-950">Team &amp; access</h1>
              <p className="mt-1 text-sm text-slate-500">Members, service accounts, and credential audit for this workspace.</p>
            </div>

            <div className="grid grid-cols-2 gap-3 sm:grid-cols-4">
              <SummaryMetric label="Members" value={String(members.length)} hint={`${activeMembers.length} active`} />
              <SummaryMetric label="Pending invites" value={String(pendingInvitations.length)} hint="Awaiting acceptance" />
              <SummaryMetric label="Service accounts" value={String(serviceAccounts.length)} hint={`${disabledServiceAccounts.length} disabled`} />
              <SummaryMetric label="Active credentials" value={String(activeCredentialCount)} hint="Machine secrets currently usable" />
            </div>

            <div className="overflow-hidden rounded-xl border border-stone-200 bg-white shadow-sm">
              <div className="flex border-b border-stone-200" role="tablist">
                {(
                  [
                    { id: "members", label: "Members", Icon: UserRound },
                    { id: "service-accounts", label: "Service accounts", Icon: ServerCog },
                    { id: "audit", label: "Audit log", Icon: ShieldCheck },
                  ] as const
                ).map((tab) => (
                  <button
                    key={tab.id}
                    type="button"
                    role="tab"
                    aria-selected={activeTab === tab.id}
                    onClick={() => setActiveTab(tab.id)}
                    className={`flex items-center gap-2 border-b-2 px-5 py-3.5 text-sm font-medium transition ${
                      activeTab === tab.id
                        ? "border-slate-900 text-slate-900"
                        : "border-transparent text-slate-500 hover:border-stone-300 hover:text-slate-700"
                    }`}
                  >
                    <tab.Icon className="h-3.5 w-3.5" />
                    {tab.label}
                  </button>
                ))}
              </div>

              {activeTab === "members" && (
                <div className="p-6">
                  <div className="grid gap-6 xl:grid-cols-[1.25fr_0.75fr]">
                    <div className="grid gap-4">
                      <div className="overflow-hidden rounded-lg border border-stone-200">
                        <div className="flex items-center justify-between border-b border-stone-200 bg-stone-50 px-4 py-3">
                          <p className="text-sm font-medium text-slate-700">Members</p>
                          <PaginationControls page={pagedMembers.page} totalPages={pagedMembers.totalPages} onPageChange={setMemberPage} label="Members" />
                        </div>
                        {pagedMembers.items.length > 0 ? (
                          <table className="w-full text-sm">
                            <thead>
                              <tr className="border-b border-stone-100 text-left text-[11px] font-semibold uppercase tracking-[0.12em] text-slate-400">
                                <th className="px-4 py-2.5 font-semibold">Member</th>
                                <th className="px-4 py-2.5 font-semibold">Role</th>
                                <th className="px-4 py-2.5 font-semibold">Status</th>
                              </tr>
                            </thead>
                            <tbody className="divide-y divide-stone-100">
                              {pagedMembers.items.map((member) => {
                                const selected = selectedMemberIDValue === member.user_id;
                                return (
                                  <tr
                                    key={member.user_id}
                                    onClick={() => setSelectedMemberID(member.user_id)}
                                    className={`cursor-pointer transition ${selected ? "bg-sky-50" : "hover:bg-stone-50"}`}
                                  >
                                    <td className="px-4 py-3">
                                      <p className="font-medium text-slate-900">{member.display_name}</p>
                                      <p className="text-xs text-slate-500">{member.email}</p>
                                    </td>
                                    <td className="px-4 py-3 capitalize text-slate-600">{member.role}</td>
                                    <td className="px-4 py-3">
                                      <div className="flex flex-wrap gap-1.5">
                                        <StatusChip tone={member.status === "active" ? "success" : "neutral"}>{member.status}</StatusChip>
                                        {isSelfMember(member.user_id) ? <StatusChip tone="info">You</StatusChip> : null}
                                        {isLastActiveAdmin(member) ? <StatusChip tone="warning">Last admin</StatusChip> : null}
                                      </div>
                                    </td>
                                  </tr>
                                );
                              })}
                            </tbody>
                          </table>
                        ) : (
                          <p className="px-4 py-6 text-sm text-slate-500">No members yet.</p>
                        )}
                      </div>

                      <div className="overflow-hidden rounded-lg border border-stone-200">
                        <div className="flex items-center justify-between border-b border-stone-200 bg-stone-50 px-4 py-3">
                          <p className="text-sm font-medium text-slate-700">Pending invites</p>
                          <PaginationControls page={pagedInvitations.page} totalPages={pagedInvitations.totalPages} onPageChange={setInvitePage} label="Pending invites" />
                        </div>
                        {pagedInvitations.items.length > 0 ? (
                          <table className="w-full text-sm">
                            <thead>
                              <tr className="border-b border-stone-100 text-left text-[11px] font-semibold uppercase tracking-[0.12em] text-slate-400">
                                <th className="px-4 py-2.5 font-semibold">Email</th>
                                <th className="px-4 py-2.5 font-semibold">Role</th>
                                <th className="px-4 py-2.5 font-semibold">Expires</th>
                                <th className="px-4 py-2.5 font-semibold" />
                              </tr>
                            </thead>
                            <tbody className="divide-y divide-stone-100">
                              {pagedInvitations.items.map((invite) => (
                                <tr key={invite.id}>
                                  <td className="px-4 py-3 font-medium text-slate-900">{invite.email}</td>
                                  <td className="px-4 py-3 capitalize text-slate-600">{invite.role}</td>
                                  <td className="px-4 py-3 text-slate-500">{formatExactTimestamp(invite.expires_at)}</td>
                                  <td className="px-4 py-3 text-right">
                                    <button
                                      type="button"
                                      onClick={() => revokeInvitationMutation.mutate(invite.id)}
                                      disabled={!csrfToken || revokeInvitationMutation.isPending}
                                      className="text-xs font-medium text-rose-600 hover:text-rose-800 disabled:cursor-not-allowed disabled:opacity-50"
                                    >
                                      Revoke
                                    </button>
                                  </td>
                                </tr>
                              ))}
                            </tbody>
                          </table>
                        ) : (
                          <p className="px-4 py-6 text-sm text-slate-500">No pending workspace invites.</p>
                        )}
                      </div>
                    </div>

                    <div className="grid gap-4 xl:sticky xl:top-6 xl:self-start">
                      <div className="rounded-lg border border-stone-200 p-4">
                        <p className="text-sm font-semibold text-slate-900">Invite a member</p>
                        <div className="mt-3 grid gap-2">
                          <input
                            {...registerInvite("email")}
                            type="email"
                            placeholder="teammate@example.com"
                            className="h-10 rounded-lg border border-stone-200 bg-white px-3 text-sm text-slate-900 outline-none ring-slate-400 transition focus:ring-2"
                          />
                          <select
                            {...registerInvite("role")}
                            aria-label="Workspace role"
                            className="h-10 rounded-lg border border-stone-200 bg-white px-3 text-sm text-slate-900 outline-none ring-slate-400 transition focus:ring-2"
                          >
                            <option value="admin">Admin</option>
                            <option value="writer">Writer</option>
                            <option value="reader">Reader</option>
                          </select>
                          <button
                            type="button"
                            onClick={handleInviteSubmit((data) => createInvitationMutation.mutate(data))}
                            disabled={!csrfToken || createInvitationMutation.isPending}
                            className="inline-flex h-10 items-center justify-center gap-2 rounded-lg border border-slate-900 bg-slate-900 px-4 text-sm font-medium text-white transition hover:bg-slate-800 disabled:cursor-not-allowed disabled:opacity-50"
                          >
                            {createInvitationMutation.isPending ? <LoaderCircle className="h-4 w-4 animate-spin" /> : <MailPlus className="h-4 w-4" />}
                            Send invite
                          </button>
                        </div>
                        {latestInviteURL ? (
                          <div className="mt-3 rounded-lg border border-stone-200 bg-stone-50 px-3 py-3">
                            <p className="text-xs text-slate-500">Share with the invitee</p>
                            <p className="mt-1.5 break-all text-xs text-slate-700">{latestInviteURL}</p>
                            <button
                              type="button"
                              onClick={() => { void navigator.clipboard.writeText(latestInviteURL); }}
                              className="mt-2 inline-flex h-8 items-center gap-1.5 rounded-lg border border-stone-200 bg-white px-3 text-xs text-slate-700 transition hover:bg-stone-100"
                            >
                              <Copy className="h-3 w-3" />
                              Copy link
                            </button>
                          </div>
                        ) : null}
                      </div>

                      {selectedMember ? (
                        <div className="rounded-lg border border-stone-200 p-4">
                          {(() => {
                            const draftRole = memberDraftRoles[selectedMember.user_id] ?? (selectedMember.role as "reader" | "writer" | "admin");
                            const roleDirty = draftRole !== selectedMember.role;
                            const selfMember = isSelfMember(selectedMember.user_id);
                            const lastAdminProtected = isLastActiveAdmin(selectedMember);
                            const showSuspendConfirm = confirmingMemberAction?.userID === selectedMember.user_id && confirmingMemberAction.action === "suspend";
                            const roleSelectDisabled = selectedMember.status !== "active" || updateMemberMutation.isPending || selfMember || lastAdminProtected;
                            const canApplyRole = roleDirty && !roleSelectDisabled && Boolean(csrfToken);
                            const canSuspend = selectedMember.status === "active" && Boolean(csrfToken) && !removeMemberMutation.isPending && !selfMember && !lastAdminProtected;
                            const canReactivate = selectedMember.status !== "active" && Boolean(csrfToken) && !updateMemberMutation.isPending && !selfMember;
                            return (
                              <>
                                <div className="flex items-start justify-between gap-3">
                                  <div className="min-w-0">
                                    <p className="font-semibold text-slate-900">{selectedMember.display_name}</p>
                                    <p className="mt-0.5 truncate text-xs text-slate-500">{selectedMember.email}</p>
                                  </div>
                                  <StatusChip tone={selectedMember.status === "active" ? "success" : "neutral"}>{selectedMember.status}</StatusChip>
                                </div>
                                <div className="mt-3 grid gap-2">
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
                                    className="h-10 rounded-lg border border-stone-200 bg-white px-3 text-sm text-slate-900 outline-none ring-slate-400 transition focus:ring-2"
                                  >
                                    <option value="admin">Admin</option>
                                    <option value="writer">Writer</option>
                                    <option value="reader">Reader</option>
                                  </select>
                                  {roleDirty ? (
                                    <div className="flex gap-2">
                                      <button type="button" onClick={() => updateMemberMutation.mutate({ userID: selectedMember.user_id, role: draftRole })} disabled={!canApplyRole} className="flex-1 inline-flex h-9 items-center justify-center rounded-lg border border-slate-900 bg-slate-900 text-sm font-medium text-white transition hover:bg-slate-800 disabled:opacity-50">Apply role</button>
                                      <button type="button" onClick={() => setMemberDraftRoles((current) => { const next = { ...current }; delete next[selectedMember.user_id]; return next; })} className="flex-1 inline-flex h-9 items-center justify-center rounded-lg border border-stone-200 text-sm text-slate-700 hover:bg-stone-100">Cancel</button>
                                    </div>
                                  ) : selectedMember.status === "active" ? (
                                    showSuspendConfirm ? (
                                      <div className="flex gap-2">
                                        <button type="button" onClick={() => removeMemberMutation.mutate(selectedMember.user_id)} disabled={!canSuspend} className="flex-1 inline-flex h-9 items-center justify-center rounded-lg border border-rose-700 bg-rose-700 text-sm font-medium text-white hover:bg-rose-800 disabled:opacity-50">Confirm suspend</button>
                                        <button type="button" onClick={() => setConfirmingMemberAction(null)} className="flex-1 inline-flex h-9 items-center justify-center rounded-lg border border-stone-200 text-sm text-slate-700 hover:bg-stone-100">Cancel</button>
                                      </div>
                                    ) : (
                                      <button type="button" onClick={() => setConfirmingMemberAction({ userID: selectedMember.user_id, action: "suspend" })} disabled={!canSuspend} className="inline-flex h-9 w-full items-center justify-center gap-2 rounded-lg border border-rose-200 bg-rose-50 text-sm font-medium text-rose-700 hover:bg-rose-100 disabled:opacity-50">
                                        <UserX className="h-4 w-4" />
                                        Suspend
                                      </button>
                                    )
                                  ) : (
                                    <button type="button" onClick={() => updateMemberMutation.mutate({ userID: selectedMember.user_id, role: selectedMember.role as "reader" | "writer" | "admin" })} disabled={!canReactivate} className="inline-flex h-9 w-full items-center justify-center rounded-lg bg-slate-900 text-sm font-medium text-white hover:bg-slate-800 disabled:opacity-50">Reactivate</button>
                                  )}
                                </div>
                                {selfMember ? <p className="mt-2 text-xs text-slate-500">You cannot change your own membership from this screen.</p> : null}
                                {lastAdminProtected ? <p className="mt-2 text-xs text-slate-500">Promote another active admin before changing this member&apos;s access.</p> : null}
                              </>
                            );
                          })()}
                        </div>
                      ) : null}
                    </div>
                  </div>
                </div>
              )}

              {activeTab === "service-accounts" && (
                <>
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

                  <div className={`flex ${selectedServiceAccount ? "divide-x divide-stone-200" : ""}`}>
                    <div className={`min-w-0 ${selectedServiceAccount ? "w-[58%]" : "w-full"}`}>
                      <div className="flex items-center justify-between border-b border-stone-200 px-6 py-4">
                        <div>
                          <p className="text-sm font-semibold text-slate-900">Service accounts</p>
                          <p className="mt-0.5 text-xs text-slate-500">API identities for automation and integrations. Issue or rotate credentials as needed.</p>
                        </div>
                        <div className="flex items-center gap-2">
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
                            <tr className="border-b border-stone-100 text-left text-[11px] font-semibold uppercase tracking-[0.12em] text-slate-400">
                              <th className="px-6 py-2.5 font-semibold">Name</th>
                              {!selectedServiceAccount && <th className="px-4 py-2.5 font-semibold">Role</th>}
                              {!selectedServiceAccount && <th className="px-4 py-2.5 font-semibold">Env</th>}
                              <th className="px-4 py-2.5 font-semibold">Status</th>
                              <th className="px-4 py-2.5 font-semibold">Credentials</th>
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
                                  {!selectedServiceAccount && (
                                    <td className="px-4 py-3.5 text-slate-600">{formatServiceAccountRole(account.role)}</td>
                                  )}
                                  {!selectedServiceAccount && (
                                    <td className="px-4 py-3.5 text-slate-500">{account.environment || "—"}</td>
                                  )}
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
                          <button
                            type="button"
                            onClick={() => { setActiveTab("audit"); openAudit(selectedServiceAccount.id); }}
                            className="inline-flex h-8 items-center gap-1.5 rounded-lg border border-stone-200 bg-white px-3 text-xs font-medium text-slate-600 transition hover:bg-stone-100"
                          >
                            <ShieldCheck className="h-3 w-3" />
                            Audit log
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
                                        <button type="button" onClick={() => revokeCredentialMutation.mutate({ serviceAccountID: selectedServiceAccount.id, credentialID: credential.id })} disabled={!csrfToken || revokeCredentialMutation.isPending} className="inline-flex h-7 items-center rounded border border-rose-200 bg-rose-50 px-2 text-[11px] font-medium text-rose-700 transition hover:bg-rose-100 disabled:opacity-50">Revoke</button>
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
              )}

              {activeTab === "audit" && (
                <div className="p-6">
                  <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
                    <div>
                      <p className="text-sm font-semibold text-slate-900">Credential audit log</p>
                      <p className="text-xs text-slate-500">Credential issue, rotation, and revocation events.</p>
                    </div>
                    <div className="flex flex-wrap gap-2">
                      <select
                        aria-label="Audit service account"
                        value={selectedAuditServiceAccountIDValue}
                        onChange={(event) => setSelectedAuditServiceAccountID(event.target.value)}
                        className="h-9 rounded-lg border border-stone-200 bg-white px-3 text-sm text-slate-800 outline-none ring-slate-400 transition focus:ring-2"
                      >
                        {serviceAccounts.map((account) => (
                          <option key={account.id} value={account.id}>{account.name}</option>
                        ))}
                      </select>
                      <button
                        type="button"
                        onClick={() => downloadAuditCSV(selectedAuditServiceAccountIDValue)}
                        disabled={!selectedAuditServiceAccountIDValue}
                        className="inline-flex h-9 items-center gap-2 rounded-lg border border-stone-200 bg-white px-3 text-sm font-medium text-slate-700 transition hover:bg-stone-100 disabled:cursor-not-allowed disabled:opacity-50"
                      >
                        <Download className="h-3.5 w-3.5" />
                        Export CSV
                      </button>
                    </div>
                  </div>

                  {selectedAuditServiceAccount ? (
                    <div className="mt-4 grid gap-4 xl:grid-cols-[1.1fr_0.9fr]">
                      <div className="overflow-hidden rounded-lg border border-stone-200">
                        <div className="flex items-center justify-between border-b border-stone-200 bg-stone-50 px-4 py-3">
                          <p className="text-sm font-medium text-slate-700">Recent events</p>
                          <CursorPaginationControls page={auditPage} hasPreviousPage={auditHasPreviousPage} hasNextPage={auditHasNextPage} onPrevious={goToPreviousAuditPage} onNext={goToNextAuditPage} label="Audit events" />
                        </div>
                        <div className="divide-y divide-stone-100">
                          {auditItems.length > 0 ? (
                            auditItems.map((event) => (
                              <ServiceAccountAuditRow key={event.id} event={event} selected={event.id === selectedAuditEventIDValue} onSelect={() => setSelectedAuditEventID(event.id)} />
                            ))
                          ) : (
                            <p className="px-4 py-6 text-sm text-slate-500">No audit events yet.</p>
                          )}
                        </div>
                      </div>

                      <div className="grid gap-4">
                        <ServiceAccountAuditDetail event={selectedAuditEvent} />
                        <div className="overflow-hidden rounded-lg border border-stone-200">
                          <div className="flex items-center justify-between border-b border-stone-200 bg-stone-50 px-4 py-3">
                            <p className="text-sm font-medium text-slate-700">Exports</p>
                            <CursorPaginationControls page={auditExportPage} hasPreviousPage={auditExportHasPreviousPage} hasNextPage={auditExportHasNextPage} onPrevious={goToPreviousAuditExportPage} onNext={goToNextAuditExportPage} label="Audit exports" />
                          </div>
                          <div className="divide-y divide-stone-100">
                            {auditExportItems.length > 0 ? (
                              auditExportItems.map((item) => (
                                <div key={item.job.id} className="flex items-center justify-between gap-4 px-4 py-3">
                                  <div className="flex items-center gap-3">
                                    <StatusChip tone={item.download_url ? "success" : item.job.status === "failed" ? "danger" : "info"}>
                                      {formatAuditExportStatus(item.job.status)}
                                    </StatusChip>
                                    <div>
                                      <p className="text-xs text-slate-700">{formatExactTimestamp(item.job.created_at)}</p>
                                      <p className="text-[11px] text-slate-500">{item.job.row_count} row(s)</p>
                                    </div>
                                  </div>
                                  {item.download_url ? (
                                    <a href={item.download_url} target="_blank" rel="noreferrer" className="inline-flex h-8 items-center gap-1.5 rounded-lg border border-stone-200 bg-white px-3 text-xs font-medium text-slate-700 transition hover:bg-stone-100">
                                      <Download className="h-3 w-3" />
                                      Download
                                    </a>
                                  ) : (
                                    <span className="text-xs text-slate-500">Pending</span>
                                  )}
                                </div>
                              ))
                            ) : (
                              <p className="px-4 py-6 text-sm text-slate-500">No exports yet.</p>
                            )}
                          </div>
                        </div>
                      </div>
                    </div>
                  ) : (
                    <p className="mt-4 text-sm text-slate-500">Create a service account first to inspect machine-credential audit.</p>
                  )}
                </div>
              )}
            </div>
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
    <div className="rounded-xl border border-stone-200 bg-white px-4 py-4 shadow-sm">
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
    <span className={`inline-flex h-7 items-center rounded-full border px-2.5 text-[11px] font-semibold ${toneClassName}`}>
      {children}
    </span>
  );
}

function ServiceAccountAuditDetail({ event }: { event: APIKeyAuditEvent | null }) {
  if (!event) {
    return (
      <div className="rounded-xl border border-dashed border-stone-300 bg-white px-4 py-6 text-sm text-slate-600">
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
    <div className="rounded-xl border border-stone-200 bg-white px-4 py-4">
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
          <DetailField label="Environment" value={environment || "Not recorded"} />
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
