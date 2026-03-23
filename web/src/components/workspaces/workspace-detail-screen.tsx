"use client";

import Link from "next/link";
import { useEffect, useState } from "react";
import { ArrowLeft, Building2, Copy, CreditCard, LoaderCircle, MailPlus, ShieldCheck, UserRound, UserX } from "lucide-react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import { LoginRedirectNotice } from "@/components/auth/login-redirect-notice";
import { ScopeNotice } from "@/components/auth/scope-notice";
import { AppBreadcrumbs } from "@/components/layout/app-breadcrumbs";
import { ControlPlaneNav } from "@/components/layout/control-plane-nav";
import {
  createWorkspaceInvitation,
  fetchBillingProviderConnection,
  fetchBillingProviderConnections,
  fetchTenantOnboardingStatus,
  fetchWorkspaceInvitations,
  fetchWorkspaceMembers,
  removeWorkspaceMember,
  revokeWorkspaceInvitation,
  updateWorkspaceMember,
  updateTenantWorkspaceBilling,
} from "@/lib/api";
import { formatExactTimestamp } from "@/lib/format";
import { describeTenantMissingStep, describeTenantSectionStep, formatReadinessStatus, normalizeMissingSteps } from "@/lib/readiness";
import { useUISession } from "@/hooks/use-ui-session";

function readinessTone(status?: string): string {
  return status === "ready"
    ? "border-emerald-200 bg-emerald-50 text-emerald-700"
    : "border-amber-200 bg-amber-50 text-amber-700";
}

export function WorkspaceDetailScreen({ tenantID }: { tenantID: string }) {
  const queryClient = useQueryClient();
  const { apiBaseURL, csrfToken, isAuthenticated, isPlatformAdmin, scope, session } = useUISession();
  const [selectedConnectionID, setSelectedConnectionID] = useState("");
  const [inviteEmail, setInviteEmail] = useState("");
  const [inviteRole, setInviteRole] = useState<"reader" | "writer" | "admin">("admin");
  const [latestInviteURL, setLatestInviteURL] = useState("");
  const [overrideReason, setOverrideReason] = useState("");
  const [memberDraftRoles, setMemberDraftRoles] = useState<Record<string, "reader" | "writer" | "admin">>({});
  const [confirmingMemberAction, setConfirmingMemberAction] = useState<{ userID: string; action: "suspend" } | null>(null);

  const tenantStatusQuery = useQuery({
    queryKey: ["tenant-onboarding-status", apiBaseURL, tenantID],
    queryFn: () => fetchTenantOnboardingStatus({ runtimeBaseURL: apiBaseURL, tenantID }),
    enabled: isAuthenticated && isPlatformAdmin && tenantID.trim().length > 0,
  });

  const selectedTenant = tenantStatusQuery.data?.tenant ?? null;
  const selectedReadiness = tenantStatusQuery.data?.readiness ?? null;
  const workspaceBilling = selectedTenant?.workspace_billing ?? null;
  const activeBillingConnectionID = workspaceBilling?.active_billing_connection_id || selectedTenant?.billing_provider_connection_id || "";

  const billingConnectionQuery = useQuery({
    queryKey: ["billing-provider-connection", apiBaseURL, activeBillingConnectionID],
    queryFn: () => fetchBillingProviderConnection({ runtimeBaseURL: apiBaseURL, connectionID: activeBillingConnectionID }),
    enabled: isAuthenticated && isPlatformAdmin && Boolean(activeBillingConnectionID),
  });

  const billingConnectionsQuery = useQuery({
    queryKey: ["billing-provider-connections", apiBaseURL, "workspace-detail"],
    queryFn: () => fetchBillingProviderConnections({ runtimeBaseURL: apiBaseURL, limit: 100, status: "connected", scope: "platform" }),
    enabled: isAuthenticated && isPlatformAdmin,
  });

  const workspaceMembersQuery = useQuery({
    queryKey: ["workspace-members", apiBaseURL, tenantID],
    queryFn: () => fetchWorkspaceMembers({ runtimeBaseURL: apiBaseURL, tenantID }),
    enabled: isAuthenticated && isPlatformAdmin && tenantID.trim().length > 0,
  });

  const workspaceInvitationsQuery = useQuery({
    queryKey: ["workspace-invitations", apiBaseURL, tenantID],
    queryFn: () => fetchWorkspaceInvitations({ runtimeBaseURL: apiBaseURL, tenantID }),
    enabled: isAuthenticated && isPlatformAdmin && tenantID.trim().length > 0,
  });

  useEffect(() => {
    setSelectedConnectionID(activeBillingConnectionID);
  }, [activeBillingConnectionID]);

  const updateWorkspaceBillingMutation = useMutation({
    mutationFn: () =>
      updateTenantWorkspaceBilling({
        runtimeBaseURL: apiBaseURL,
        csrfToken,
        tenantID,
        billingProviderConnectionID: selectedConnectionID,
      }),
    onSuccess: async () => {
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ["tenant-onboarding-status", apiBaseURL, tenantID] }),
        queryClient.invalidateQueries({ queryKey: ["tenants"] }),
        queryClient.invalidateQueries({ queryKey: ["overview-tenants"] }),
        queryClient.invalidateQueries({ queryKey: ["billing-provider-connection"] }),
      ]);
    },
  });

  const createInvitationMutation = useMutation({
    mutationFn: () =>
      createWorkspaceInvitation({
        runtimeBaseURL: apiBaseURL,
        csrfToken,
        tenantID,
        email: inviteEmail,
        role: inviteRole,
      }),
    onSuccess: async () => {
      setInviteEmail("");
      setInviteRole("admin");
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ["workspace-invitations", apiBaseURL, tenantID] }),
        queryClient.invalidateQueries({ queryKey: ["workspace-members", apiBaseURL, tenantID] }),
      ]);
    },
  });

  const revokeInvitationMutation = useMutation({
    mutationFn: (invitationID: string) =>
      revokeWorkspaceInvitation({
        runtimeBaseURL: apiBaseURL,
        csrfToken,
        tenantID,
        invitationID,
        reason: overrideReason,
      }),
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: ["workspace-invitations", apiBaseURL, tenantID] });
    },
  });

  const updateMemberMutation = useMutation({
    mutationFn: (input: { userID: string; role: "reader" | "writer" | "admin" }) =>
      updateWorkspaceMember({
        runtimeBaseURL: apiBaseURL,
        csrfToken,
        tenantID,
        userID: input.userID,
        role: input.role,
        reason: overrideReason,
      }),
    onSuccess: async (_payload, input) => {
      setMemberDraftRoles((current) => {
        const next = { ...current };
        delete next[input.userID];
        return next;
      });
      setConfirmingMemberAction(null);
      await queryClient.invalidateQueries({ queryKey: ["workspace-members", apiBaseURL, tenantID] });
    },
  });

  const removeMemberMutation = useMutation({
    mutationFn: (userID: string) =>
      removeWorkspaceMember({
        runtimeBaseURL: apiBaseURL,
        csrfToken,
        tenantID,
        userID,
        reason: overrideReason,
      }),
    onSuccess: async () => {
      setConfirmingMemberAction(null);
      await queryClient.invalidateQueries({ queryKey: ["workspace-members", apiBaseURL, tenantID] });
    },
  });

  const readinessMissingSteps = normalizeMissingSteps(selectedReadiness?.missing_steps);
  const tenantMissingSteps = normalizeMissingSteps(selectedReadiness?.tenant?.missing_steps);
  const billingMissingSteps = normalizeMissingSteps(selectedReadiness?.billing_integration?.missing_steps);
  const firstCustomerMissingSteps = normalizeMissingSteps(selectedReadiness?.first_customer?.missing_steps);
  const nextActions = readinessMissingSteps.map(describeTenantMissingStep);
  const availableConnections = billingConnectionsQuery.data ?? [];
  const workspaceMembers = workspaceMembersQuery.data ?? [];
  const workspaceInvitations = workspaceInvitationsQuery.data ?? [];
  const pendingInvitations = workspaceInvitations.filter((item) => item.status === "pending");
  const currentUserID = session?.subject_id ?? "";
  const activeAdminCount = workspaceMembers.filter((member) => member.status === "active" && member.role === "admin").length;
  const canSaveWorkspaceBilling =
    Boolean(csrfToken) &&
    !updateWorkspaceBillingMutation.isPending &&
    Boolean(selectedConnectionID) &&
    selectedConnectionID !== activeBillingConnectionID;
  const canCreateInvitation = Boolean(csrfToken) && !createInvitationMutation.isPending && inviteEmail.trim().length > 0;
  const canRunOverrideAction = Boolean(csrfToken) && overrideReason.trim().length > 0;

  useEffect(() => {
    if (createInvitationMutation.data?.accept_url) {
      setLatestInviteURL(createInvitationMutation.data.accept_url);
    }
  }, [createInvitationMutation.data]);

  const isSelfMember = (userID: string): boolean => currentUserID !== "" && currentUserID === userID;
  const isLastActiveAdmin = (member: { role: string; status: string }): boolean =>
    member.status === "active" && member.role === "admin" && activeAdminCount <= 1;

  return (
    <div className="min-h-screen bg-[#f5f7fb] text-slate-900">
      <main className="mx-auto flex max-w-[1360px] flex-col gap-5 px-4 py-6 md:px-6 lg:px-8">
        <ControlPlaneNav />
        <AppBreadcrumbs items={[{ href: "/billing-connections", label: "Platform" }, { href: "/workspaces", label: "Workspaces" }, { label: selectedTenant?.name || tenantID }]} />

        {!isAuthenticated ? <LoginRedirectNotice /> : null}
        {isAuthenticated && scope !== "platform" ? (
          <ScopeNotice
            title="Platform session required"
            body="Workspace detail is a platform-admin view. Sign in with a platform account to inspect cross-workspace readiness."
            actionHref="/customers"
            actionLabel="Open tenant home"
          />
        ) : null}

        {tenantStatusQuery.isLoading ? (
          <LoadingPanel label="Loading workspace detail" />
        ) : tenantStatusQuery.isError || !selectedTenant || !selectedReadiness ? (
          <section className="rounded-2xl border border-slate-200 bg-white p-6 shadow-sm">
            <p className="text-xs font-semibold uppercase tracking-[0.16em] text-slate-500">Workspace</p>
            <h1 className="mt-2 text-2xl font-semibold text-slate-950">Workspace not available</h1>
            <p className="mt-3 text-sm text-slate-600">The requested workspace could not be loaded from the onboarding status API.</p>
            <Link
              href="/workspaces"
              className="mt-5 inline-flex h-10 items-center gap-2 rounded-lg border border-slate-200 bg-slate-50 px-4 text-sm text-slate-700 transition hover:bg-slate-100"
            >
              <ArrowLeft className="h-4 w-4" />
              Back to workspaces
            </Link>
          </section>
        ) : (
          <>
            <section className="rounded-2xl border border-slate-200 bg-white p-6 shadow-sm">
              <div className="flex flex-col gap-5 lg:flex-row lg:items-start lg:justify-between">
                <div className="min-w-0">
                  <p className="text-xs font-semibold uppercase tracking-[0.16em] text-slate-500">Workspace</p>
                  <h1 className="mt-2 break-words text-3xl font-semibold tracking-tight text-slate-950">{selectedTenant.name}</h1>
                  <div className="mt-3 flex flex-wrap items-center gap-3 text-sm text-slate-600">
                    <span className="font-mono text-xs text-slate-500">{selectedTenant.id}</span>
                    <span className={`rounded-full border px-2.5 py-1 text-[11px] font-semibold uppercase tracking-[0.14em] ${readinessTone(selectedReadiness.status)}`}>
                      {formatReadinessStatus(selectedReadiness.status)}
                    </span>
                    <span className="rounded-full border border-slate-200 bg-slate-50 px-2.5 py-1 text-[11px] font-semibold uppercase tracking-[0.14em] text-slate-600">
                      {formatReadinessStatus(selectedTenant.status)}
                    </span>
                  </div>
                </div>
                <div className="flex flex-wrap items-center gap-3">
                  <Link
                    href="/workspaces"
                    className="inline-flex h-10 items-center gap-2 rounded-lg border border-slate-200 bg-slate-50 px-4 text-sm text-slate-700 transition hover:bg-slate-100"
                  >
                    <ArrowLeft className="h-4 w-4" />
                    Back to workspaces
                  </Link>
                  <Link
                    href="/workspaces/new"
                    className="inline-flex h-10 items-center gap-2 rounded-lg border border-slate-900 bg-slate-900 px-4 text-sm font-medium text-white transition hover:bg-slate-800"
                  >
                    <Building2 className="h-4 w-4" />
                    New workspace
                  </Link>
                </div>
              </div>
            </section>

            <section className="grid gap-4 md:grid-cols-2 xl:grid-cols-4">
              <SummaryStat label="Workspace" value={selectedReadiness.tenant.status} helper={selectedReadiness.tenant.tenant_active ? "Active and available" : "Needs activation"} />
              <SummaryStat
                label="Billing"
                value={selectedReadiness.billing_integration.status}
                helper={
                  selectedReadiness.billing_integration.billing_connected
                    ? `Active connection linked${selectedReadiness.billing_integration.isolation_mode ? ` · ${selectedReadiness.billing_integration.isolation_mode}` : ""}`
                    : selectedReadiness.billing_integration.pricing_ready
                      ? "Pricing ready, billing not attached"
                      : "Billing and pricing still need setup"
                }
              />
              <SummaryStat
                label="First customer"
                value={selectedReadiness.first_customer.status}
                helper={selectedReadiness.first_customer.customer_exists ? "Customer exists" : "No customer yet"}
              />
              <SummaryStat label="Open actions" value={String(readinessMissingSteps.length)} helper="Remaining checklist items" />
            </section>

            <div className="grid gap-5 xl:grid-cols-[minmax(0,1.2fr)_420px]">
              <div className="grid gap-5">
                <section className="rounded-2xl border border-slate-200 bg-white p-6 shadow-sm">
                  <div className="flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between">
                    <div>
                      <p className="text-xs font-semibold uppercase tracking-[0.16em] text-slate-500">Readiness</p>
                      <h2 className="mt-2 text-xl font-semibold text-slate-950">Operational checklist</h2>
                      <p className="mt-2 text-sm text-slate-600">Track what still blocks handoff into normal tenant operations.</p>
                    </div>
                    <div className="rounded-xl border border-slate-200 bg-slate-50 px-4 py-3 text-sm text-slate-600">
                      {nextActions.length === 0 ? "No remaining blockers" : `${nextActions.length} action item(s) remaining`}
                    </div>
                  </div>
                  <div className="mt-5 grid gap-3">
                    {nextActions.length > 0 ? nextActions.map((item) => <ChecklistLine key={item} done={false} text={item} />) : <ChecklistLine done text="Workspace is ready for the next operational handoff." />}
                  </div>
                  <div className="mt-5 grid gap-3 lg:grid-cols-3">
                    <ReadinessCard title="Workspace" readiness={selectedReadiness.tenant.status} missing={tenantMissingSteps} />
                    <ReadinessCard title="Billing integration" readiness={selectedReadiness.billing_integration.status} missing={billingMissingSteps} />
                    <ReadinessCard title="First customer" readiness={selectedReadiness.first_customer.status} missing={firstCustomerMissingSteps} />
                  </div>
                </section>

                <section className="rounded-2xl border border-slate-200 bg-white p-6 shadow-sm">
                  <div className="flex items-start justify-between gap-4">
                    <div>
                      <p className="text-xs font-semibold uppercase tracking-[0.16em] text-slate-500">Workspace access</p>
                      <h2 className="mt-2 text-xl font-semibold text-slate-950">Members and pending invites</h2>
                    </div>
                    <div className="grid min-w-[180px] gap-2 text-sm text-slate-600">
                      <InlineStat label="Members" value={String(workspaceMembers.length)} />
                      <InlineStat label="Pending" value={String(pendingInvitations.length)} />
                    </div>
                  </div>

                  <div className="mt-5 grid gap-5 xl:grid-cols-[320px_minmax(0,1fr)]">
                    <section className="rounded-xl border border-slate-200 bg-slate-50 p-4">
                      <p className="text-sm font-semibold text-slate-950">Invite workspace operator</p>
                      <p className="mt-2 text-xs leading-relaxed text-slate-600">
                        Hand off this workspace through Alpha access, not backend-only provisioning.
                      </p>
                      <div className="mt-4 grid gap-3">
                        <input
                          type="email"
                          value={inviteEmail}
                          onChange={(event) => setInviteEmail(event.target.value)}
                          placeholder="tenant-admin@example.com"
                          className="h-10 rounded-lg border border-slate-200 bg-white px-3 text-sm text-slate-900 outline-none ring-slate-400 transition placeholder:text-slate-400 focus:ring-2"
                        />
                        <select
                          aria-label="Workspace role"
                          value={inviteRole}
                          onChange={(event) => setInviteRole(event.target.value as "reader" | "writer" | "admin")}
                          className="h-10 rounded-lg border border-slate-200 bg-white px-3 text-sm text-slate-900 outline-none ring-slate-400 transition focus:ring-2"
                        >
                          <option value="admin">Admin</option>
                          <option value="writer">Writer</option>
                          <option value="reader">Reader</option>
                        </select>
                        <button
                          type="button"
                          onClick={() => createInvitationMutation.mutate()}
                          disabled={!canCreateInvitation}
                          className="inline-flex h-10 items-center justify-center gap-2 rounded-lg border border-slate-900 bg-slate-900 px-4 text-sm font-medium text-white transition hover:bg-slate-800 disabled:cursor-not-allowed disabled:opacity-50"
                        >
                          {createInvitationMutation.isPending ? <LoaderCircle className="h-4 w-4 animate-spin" /> : <MailPlus className="h-4 w-4" />}
                          Send invite
                        </button>
                      </div>
                      {latestInviteURL ? (
                        <div className="mt-4 rounded-lg border border-slate-200 bg-white p-3">
                          <p className="text-[11px] font-semibold uppercase tracking-[0.14em] text-slate-500">Latest invite link</p>
                          <p className="mt-2 break-all text-xs text-slate-700">{latestInviteURL}</p>
                          <button
                            type="button"
                            onClick={() => {
                              void navigator.clipboard.writeText(latestInviteURL);
                            }}
                            className="mt-3 inline-flex h-9 items-center gap-2 rounded-lg border border-slate-200 bg-slate-50 px-3 text-xs text-slate-700 transition hover:bg-slate-100"
                          >
                            <Copy className="h-3.5 w-3.5" />
                            Copy invite link
                          </button>
                        </div>
                      ) : null}
                    </section>

                    <div className="grid gap-4">
                      <section className="rounded-xl border border-amber-200 bg-amber-50 p-4">
                        <p className="text-sm font-semibold text-slate-950">Platform support override</p>
                        <p className="mt-2 text-xs leading-relaxed text-slate-600">
                          Platform admins can override tenant access for support, recovery, and compliance. Member changes and invite revocation require a reason and are audited.
                        </p>
                        <div className="mt-4 grid gap-2">
                          <label className="text-[11px] font-semibold uppercase tracking-[0.14em] text-slate-500">Override reason</label>
                          <input
                            type="text"
                            value={overrideReason}
                            onChange={(event) => setOverrideReason(event.target.value)}
                            placeholder="Support case, recovery action, or compliance reason"
                            className="h-10 rounded-lg border border-slate-200 bg-white px-3 text-sm text-slate-900 outline-none ring-slate-400 transition placeholder:text-slate-400 focus:ring-2"
                          />
                        </div>
                      </section>

                      <section className="rounded-xl border border-slate-200 bg-slate-50 p-4">
                        <p className="text-sm font-semibold text-slate-950">Current members</p>
                        <div className="mt-3 grid gap-3">
                          {workspaceMembers.length > 0 ? (
                            workspaceMembers.map((member) => (
                              <div key={member.user_id} className="grid gap-3 rounded-lg border border-slate-200 bg-white px-4 py-3 lg:grid-cols-[minmax(0,1fr)_minmax(0,320px)] lg:items-center">
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
                                    roleDirty && !roleSelectDisabled && canRunOverrideAction;
                                  const canSuspend =
                                    member.status === "active" &&
                                    canRunOverrideAction &&
                                    !removeMemberMutation.isPending &&
                                    !selfMember &&
                                    !lastAdminProtected;
                                  const canReactivate =
                                    member.status !== "active" && canRunOverrideAction && !updateMemberMutation.isPending && !selfMember;

                                  return (
                                    <>
                                      <div className="min-w-0">
                                        <p className="flex items-center gap-2 text-sm font-medium text-slate-950">
                                          <UserRound className="h-4 w-4 text-slate-500" />
                                          <span className="truncate">{member.display_name}</span>
                                        </p>
                                        <p className="mt-1 break-all text-xs text-slate-600">{member.email}</p>
                                        <div className="mt-2 flex flex-wrap items-center gap-2">
                                          <span className="rounded-full border border-slate-200 bg-slate-50 px-2.5 py-1 text-center text-[11px] font-semibold uppercase tracking-[0.14em] text-slate-600">
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
                                          <p className="mt-2 text-xs text-slate-500">Platform overrides cannot change the acting user&apos;s own membership.</p>
                                        ) : null}
                                        {lastAdminProtected ? (
                                          <p className="mt-2 text-xs text-slate-500">Assign another active admin before changing this member&apos;s access.</p>
                                        ) : null}
                                      </div>
                                      <div className="flex flex-wrap items-center justify-end gap-2">
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
                                          className="h-10 rounded-lg border border-slate-200 bg-white px-3 text-xs uppercase tracking-[0.12em] text-slate-800 outline-none ring-slate-400 transition focus:ring-2 disabled:cursor-not-allowed disabled:opacity-50"
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
                                              className="inline-flex h-10 items-center gap-2 rounded-lg border border-slate-900 bg-slate-900 px-3 text-xs uppercase tracking-[0.12em] text-white transition hover:bg-slate-800 disabled:cursor-not-allowed disabled:opacity-50"
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
                                              className="inline-flex h-10 items-center gap-2 rounded-lg border border-slate-200 bg-white px-3 text-xs uppercase tracking-[0.12em] text-slate-700 transition hover:bg-slate-100"
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
                                                className="inline-flex h-10 items-center gap-2 rounded-lg border border-rose-700 bg-rose-700 px-3 text-xs uppercase tracking-[0.12em] text-white transition hover:bg-rose-800 disabled:cursor-not-allowed disabled:opacity-50"
                                              >
                                                Confirm suspend
                                              </button>
                                              <button
                                                type="button"
                                                onClick={() => setConfirmingMemberAction(null)}
                                                className="inline-flex h-10 items-center gap-2 rounded-lg border border-slate-200 bg-white px-3 text-xs uppercase tracking-[0.12em] text-slate-700 transition hover:bg-slate-100"
                                              >
                                                Cancel
                                              </button>
                                            </>
                                          ) : (
                                            <button
                                              type="button"
                                              onClick={() => setConfirmingMemberAction({ userID: member.user_id, action: "suspend" })}
                                              disabled={!canSuspend}
                                              className="inline-flex h-10 items-center gap-2 rounded-lg border border-rose-200 bg-rose-50 px-3 text-xs uppercase tracking-[0.12em] text-rose-700 transition hover:bg-rose-100 disabled:cursor-not-allowed disabled:opacity-50"
                                            >
                                              <UserX className="h-3.5 w-3.5" />
                                              Suspend
                                            </button>
                                          )
                                        ) : (
                                          <button
                                            type="button"
                                            onClick={() =>
                                              updateMemberMutation.mutate({
                                                userID: member.user_id,
                                                role: member.role as "reader" | "writer" | "admin",
                                              })
                                            }
                                            disabled={!canReactivate}
                                            className="inline-flex h-10 items-center gap-2 rounded-lg border border-emerald-200 bg-emerald-50 px-3 text-xs uppercase tracking-[0.12em] text-emerald-700 transition hover:bg-emerald-100 disabled:cursor-not-allowed disabled:opacity-50"
                                          >
                                            Reactivate
                                          </button>
                                        )}
                                      </div>
                                    </>
                                  );
                                })()}
                              </div>
                            ))
                          ) : (
                            <EmptyPanel message="No members yet. Invite the first workspace admin to complete the handoff." />
                          )}
                        </div>
                      </section>

                      <section className="rounded-xl border border-slate-200 bg-slate-50 p-4">
                        <p className="text-sm font-semibold text-slate-950">Pending invites</p>
                        <div className="mt-3 grid gap-3">
                          {pendingInvitations.length > 0 ? (
                            pendingInvitations.map((invite) => (
                              <div key={invite.id} className="grid gap-3 rounded-lg border border-slate-200 bg-white px-4 py-3 lg:grid-cols-[minmax(0,1fr)_96px] lg:items-center">
                                <div className="min-w-0">
                                  <p className="flex items-center gap-2 text-sm font-medium text-slate-950">
                                    <ShieldCheck className="h-4 w-4 text-amber-600" />
                                    <span className="truncate">{invite.email}</span>
                                  </p>
                                  <p className="mt-1 text-xs text-slate-600">
                                    {invite.role} · expires {formatExactTimestamp(invite.expires_at)}
                                  </p>
                                </div>
                                <button
                                  type="button"
                                  onClick={() => revokeInvitationMutation.mutate(invite.id)}
                                  disabled={!canRunOverrideAction || revokeInvitationMutation.isPending}
                                  className="inline-flex h-9 items-center justify-center rounded-lg border border-slate-200 bg-slate-50 px-3 text-xs text-slate-700 transition hover:bg-slate-100 disabled:cursor-not-allowed disabled:opacity-50"
                                >
                                  Revoke
                                </button>
                              </div>
                            ))
                          ) : (
                            <EmptyPanel message="No pending workspace invites." />
                          )}
                        </div>
                      </section>
                    </div>
                  </div>
                </section>
              </div>

              <aside className="grid gap-5 self-start">
                <section className="rounded-2xl border border-slate-200 bg-white p-6 shadow-sm">
                  <div className="flex items-start justify-between gap-4">
                    <div>
                      <p className="text-xs font-semibold uppercase tracking-[0.16em] text-slate-500">Workspace billing</p>
                      <h2 className="mt-2 text-xl font-semibold text-slate-950">Active billing path</h2>
                    </div>
                    <span className={`rounded-full border px-2.5 py-1 text-[11px] font-semibold uppercase tracking-[0.14em] ${readinessTone(workspaceBilling?.status || selectedReadiness.billing_integration.workspace_billing_status || selectedReadiness.billing_integration.status)}`}>
                      {formatReadinessStatus(workspaceBilling?.status || selectedReadiness.billing_integration.workspace_billing_status || selectedReadiness.billing_integration.status)}
                    </span>
                  </div>

                  <div className="mt-4 grid gap-3">
                    <MetaItem label="Active connection" value={activeBillingConnectionID || "Not assigned"} mono={Boolean(activeBillingConnectionID)} />
                    <MetaItem label="Connection name" value={billingConnectionQuery.data?.display_name || (billingConnectionQuery.isLoading ? "Loading" : "Unavailable")} />
                    <MetaItem label="Connection status" value={billingConnectionQuery.data ? formatReadinessStatus(billingConnectionQuery.data.status) : billingConnectionQuery.isLoading ? "Loading" : "Unavailable"} />
                    <MetaItem label="Isolation mode" value={workspaceBilling?.isolation_mode ? formatReadinessStatus(workspaceBilling.isolation_mode) : selectedReadiness.billing_integration.isolation_mode ? formatReadinessStatus(selectedReadiness.billing_integration.isolation_mode) : "Shared"} />
                    <MetaItem label="Binding source" value={workspaceBilling?.source || selectedReadiness.billing_integration.workspace_billing_source || "Pending binding"} />
                  </div>

                  {activeBillingConnectionID ? (
                    <Link
                      href={`/billing-connections/${encodeURIComponent(activeBillingConnectionID)}`}
                      className="mt-4 inline-flex h-10 items-center justify-center gap-2 rounded-lg border border-slate-200 bg-slate-50 px-4 text-sm text-slate-700 transition hover:bg-slate-100"
                    >
                      <CreditCard className="h-4 w-4" />
                      Open billing connection
                    </Link>
                  ) : null}

                  <div className="mt-5 rounded-xl border border-slate-200 bg-slate-50 p-4">
                    <p className="text-sm font-semibold text-slate-950">Change active connection</p>
                    <div className="mt-3 grid gap-3">
                      <select
                        aria-label="Active billing connection"
                        value={selectedConnectionID}
                        onChange={(event) => setSelectedConnectionID(event.target.value)}
                        className="h-10 rounded-lg border border-slate-200 bg-white px-3 text-sm text-slate-900 outline-none ring-slate-400 transition focus:ring-2"
                      >
                        <option value="">Select one active billing connection</option>
                        {availableConnections.map((connection) => (
                          <option key={connection.id} value={connection.id}>
                            {connection.display_name} · {connection.environment}
                          </option>
                        ))}
                      </select>
                      <button
                        type="button"
                        onClick={() => updateWorkspaceBillingMutation.mutate()}
                        disabled={!canSaveWorkspaceBilling}
                        className="inline-flex h-10 items-center justify-center gap-2 rounded-lg border border-slate-900 bg-slate-900 px-4 text-sm font-medium text-white transition hover:bg-slate-800 disabled:cursor-not-allowed disabled:opacity-50"
                      >
                        {updateWorkspaceBillingMutation.isPending ? <LoaderCircle className="h-4 w-4 animate-spin" /> : <CreditCard className="h-4 w-4" />}
                        Save active connection
                      </button>
                    </div>
                  </div>
                </section>

                <section className="rounded-2xl border border-slate-200 bg-white p-6 shadow-sm">
                  <p className="text-xs font-semibold uppercase tracking-[0.16em] text-slate-500">Metadata</p>
                  <div className="mt-4 grid gap-3">
                    <MetaItem label="Created" value={formatExactTimestamp(selectedTenant.created_at)} />
                    <MetaItem label="Updated" value={formatExactTimestamp(selectedTenant.updated_at)} />
                    <MetaItem label="Workspace status" value={formatReadinessStatus(selectedTenant.status)} />
                  </div>
                </section>
              </aside>
            </div>
          </>
        )}
      </main>
    </div>
  );
}

function LoadingPanel({ label }: { label: string }) {
  return (
    <section className="rounded-2xl border border-slate-200 bg-white p-6 text-sm text-slate-600 shadow-sm">
      <div className="flex items-center gap-2">
        <LoaderCircle className="h-4 w-4 animate-spin" />
        {label}
      </div>
    </section>
  );
}

function SummaryStat({ label, value, helper }: { label: string; value: string; helper: string }) {
  return (
    <div className="rounded-2xl border border-slate-200 bg-white px-4 py-4 shadow-sm">
      <p className="text-[11px] font-semibold uppercase tracking-[0.15em] text-slate-500">{label}</p>
      <p className="mt-2 text-base font-semibold text-slate-950">{formatReadinessStatus(value)}</p>
      <p className="mt-2 text-xs leading-relaxed text-slate-600">{helper}</p>
    </div>
  );
}

function ReadinessCard({ title, readiness, missing }: { title: string; readiness: string; missing?: string[] | null }) {
  const missingSteps = normalizeMissingSteps(missing);
  const lead = missingSteps[0] ? describeTenantSectionStep(missingSteps[0]) : "No action needed";
  return (
    <div className="rounded-xl border border-slate-200 bg-slate-50 p-4">
      <div className="flex items-center justify-between gap-3">
        <p className="text-sm font-semibold text-slate-950">{title}</p>
        <span className={`rounded-full border px-2 py-1 text-[11px] font-semibold uppercase tracking-[0.14em] ${readinessTone(readiness)}`}>
          {formatReadinessStatus(readiness)}
        </span>
      </div>
      <p className="mt-3 text-xs text-slate-700">{lead}</p>
      <p className="mt-2 text-xs text-slate-500">{missingSteps.length === 0 ? "All set" : `${missingSteps.length} action item(s) remaining`}</p>
    </div>
  );
}

function ChecklistLine({ done, text }: { done: boolean; text: string }) {
  return (
    <div className="flex items-start gap-3 rounded-lg border border-slate-200 bg-slate-50 px-3 py-3">
      <span
        className={`mt-0.5 inline-flex h-5 w-5 items-center justify-center rounded-full text-[11px] font-semibold ${done ? "bg-emerald-100 text-emerald-700" : "bg-amber-100 text-amber-700"}`}
      >
        {done ? "OK" : "!"}
      </span>
      <p className="text-sm text-slate-800">{text}</p>
    </div>
  );
}

function MetaItem({ label, value, mono }: { label: string; value: string; mono?: boolean }) {
  return (
    <div className="rounded-xl border border-slate-200 bg-slate-50 px-4 py-3">
      <dt className="text-[11px] font-semibold uppercase tracking-[0.14em] text-slate-500">{label}</dt>
      <dd className={`mt-2 break-all text-sm text-slate-900 ${mono ? "font-mono" : ""}`}>{value}</dd>
    </div>
  );
}

function InlineStat({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex items-center justify-between rounded-lg border border-slate-200 bg-slate-50 px-3 py-2">
      <span className="text-xs font-semibold uppercase tracking-[0.14em] text-slate-500">{label}</span>
      <span className="text-sm font-semibold text-slate-950">{value}</span>
    </div>
  );
}

function EmptyPanel({ message }: { message: string }) {
  return <p className="rounded-lg border border-dashed border-slate-300 bg-white px-4 py-6 text-sm text-slate-600">{message}</p>;
}
