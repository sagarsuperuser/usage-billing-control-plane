"use client";

import { useState } from "react";
import { Copy, LoaderCircle, MailPlus, ShieldCheck, UserRound, UserX } from "lucide-react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import { LoginRedirectNotice } from "@/components/auth/login-redirect-notice";
import { ScopeNotice } from "@/components/auth/scope-notice";
import { ControlPlaneNav } from "@/components/layout/control-plane-nav";
import { AppBreadcrumbs } from "@/components/layout/app-breadcrumbs";
import {
  createTenantWorkspaceInvitation,
  fetchTenantWorkspaceInvitations,
  fetchTenantWorkspaceMembers,
  removeTenantWorkspaceMember,
  revokeTenantWorkspaceInvitation,
  updateTenantWorkspaceMember,
} from "@/lib/api";
import { formatExactTimestamp } from "@/lib/format";
import { useUISession } from "@/hooks/use-ui-session";

export function TenantWorkspaceAccessScreen() {
  const queryClient = useQueryClient();
  const { apiBaseURL, csrfToken, isAuthenticated, scope, role, isAdmin, session } = useUISession();
  const [inviteEmail, setInviteEmail] = useState("");
  const [inviteRole, setInviteRole] = useState<"reader" | "writer" | "admin">("writer");

  const membersQuery = useQuery({
    queryKey: ["tenant-workspace-members", apiBaseURL, session?.tenant_id],
    queryFn: () => fetchTenantWorkspaceMembers({ runtimeBaseURL: apiBaseURL }),
    enabled: isAuthenticated && scope === "tenant" && isAdmin,
  });
  const invitationsQuery = useQuery({
    queryKey: ["tenant-workspace-invitations", apiBaseURL, session?.tenant_id],
    queryFn: () => fetchTenantWorkspaceInvitations({ runtimeBaseURL: apiBaseURL }),
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
      await queryClient.invalidateQueries({ queryKey: ["tenant-workspace-invitations", apiBaseURL, session?.tenant_id] });
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
      await queryClient.invalidateQueries({ queryKey: ["tenant-workspace-invitations", apiBaseURL, session?.tenant_id] });
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
      await queryClient.invalidateQueries({ queryKey: ["tenant-workspace-members", apiBaseURL, session?.tenant_id] });
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
      await queryClient.invalidateQueries({ queryKey: ["tenant-workspace-members", apiBaseURL, session?.tenant_id] });
    },
  });

  const members = membersQuery.data ?? [];
  const invitations = invitationsQuery.data ?? [];
  const pendingInvitations = invitations.filter((item) => item.status === "pending");
  const latestInviteURL = createInvitationMutation.data?.accept_url ?? "";

  return (
    <div className="relative min-h-screen overflow-hidden bg-[radial-gradient(circle_at_top_right,_#172554_0%,_#0f172a_38%,_#090d16_78%)] text-slate-100">
      <main className="relative mx-auto flex max-w-[1180px] flex-col gap-6 px-4 py-6 md:px-8 lg:px-10">
        <ControlPlaneNav />
        <AppBreadcrumbs items={[{ href: "/customers", label: "Workspace" }, { label: "Access" }]} />

        {!isAuthenticated ? <LoginRedirectNotice /> : null}
        {isAuthenticated && scope !== "tenant" ? (
          <ScopeNotice
            title="Tenant session required"
            body="Workspace access management belongs inside a tenant workspace. Switch to a tenant session to manage members and invitations."
            actionHref="/billing-connections"
            actionLabel="Open platform home"
          />
        ) : null}
        {isAuthenticated && scope === "tenant" && !isAdmin ? (
          <ScopeNotice
            title="Workspace admin role required"
            body={`You are signed in as ${role || "reader"}. Only workspace admins can manage invitations, roles, and member removal.`}
            actionHref="/customers"
            actionLabel="Open workspace home"
          />
        ) : null}

        {isAuthenticated && scope === "tenant" && isAdmin ? (
          <>
            <section className="rounded-3xl border border-white/10 bg-slate-900/70 p-6 backdrop-blur-xl">
              <p className="text-xs uppercase tracking-[0.2em] text-cyan-300/80">Workspace access</p>
              <h1 className="mt-2 text-3xl font-semibold text-white">Members and invitations</h1>
              <p className="mt-3 text-sm text-slate-300">
                Tenant admins own ongoing access for this workspace. Platform setup hands off here; day-to-day member changes happen here.
              </p>
            </section>

            <section className="grid gap-4 md:grid-cols-2">
              <div className="rounded-3xl border border-white/10 bg-slate-900/70 p-6 backdrop-blur-xl">
                <p className="text-xs uppercase tracking-[0.2em] text-cyan-300/80">Invite member</p>
                <div className="mt-4 grid gap-3">
                  <input
                    type="email"
                    value={inviteEmail}
                    onChange={(event) => setInviteEmail(event.target.value)}
                    placeholder="teammate@example.com"
                    className="h-11 rounded-xl border border-white/15 bg-slate-950/70 px-3 text-sm text-slate-100 outline-none ring-cyan-400 transition focus:ring-2"
                  />
                  <select
                    aria-label="Workspace role"
                    value={inviteRole}
                    onChange={(event) => setInviteRole(event.target.value as "reader" | "writer" | "admin")}
                    className="h-11 rounded-xl border border-white/15 bg-slate-950/70 px-3 text-sm text-slate-100 outline-none ring-cyan-400 transition focus:ring-2"
                  >
                    <option value="admin">Admin</option>
                    <option value="writer">Writer</option>
                    <option value="reader">Reader</option>
                  </select>
                  <button
                    type="button"
                    onClick={() => createInvitationMutation.mutate()}
                    disabled={!csrfToken || !inviteEmail.trim() || createInvitationMutation.isPending}
                    className="inline-flex h-11 items-center justify-center gap-2 rounded-xl border border-cyan-400/40 bg-cyan-500/10 px-4 text-sm font-medium text-cyan-100 transition hover:bg-cyan-500/20 disabled:cursor-not-allowed disabled:opacity-50"
                  >
                    {createInvitationMutation.isPending ? <LoaderCircle className="h-4 w-4 animate-spin" /> : <MailPlus className="h-4 w-4" />}
                    Send invite
                  </button>
                </div>
                {latestInviteURL ? (
                  <div className="mt-4 rounded-2xl border border-white/10 bg-slate-950/55 px-4 py-3">
                    <p className="text-[11px] uppercase tracking-[0.14em] text-slate-400">Latest invite link</p>
                    <p className="mt-2 break-all text-xs text-slate-300">{latestInviteURL}</p>
                    <button
                      type="button"
                      onClick={() => {
                        void navigator.clipboard.writeText(latestInviteURL);
                      }}
                      className="mt-3 inline-flex h-9 items-center gap-2 rounded-xl border border-white/10 bg-white/5 px-3 text-xs text-slate-200 transition hover:bg-white/10"
                    >
                      <Copy className="h-3.5 w-3.5" />
                      Copy invite link
                    </button>
                  </div>
                ) : null}
              </div>

              <div className="rounded-3xl border border-white/10 bg-slate-900/70 p-6 backdrop-blur-xl">
                <p className="text-xs uppercase tracking-[0.2em] text-cyan-300/80">Pending invites</p>
                <div className="mt-4 grid gap-3">
                  {pendingInvitations.length > 0 ? (
                    pendingInvitations.map((invite) => (
                      <div key={invite.id} className="rounded-2xl border border-white/10 bg-slate-950/55 px-4 py-3">
                        <div className="flex items-start justify-between gap-3">
                          <div className="min-w-0">
                            <p className="flex items-center gap-2 text-sm font-medium text-white">
                              <ShieldCheck className="h-4 w-4 text-amber-300" />
                              <span className="truncate">{invite.email}</span>
                            </p>
                            <p className="mt-1 text-xs text-slate-400">
                              {invite.role} · expires {formatExactTimestamp(invite.expires_at)}
                            </p>
                            {invite.accept_url ? <p className="mt-2 break-all text-[11px] text-slate-500">{invite.accept_url}</p> : null}
                          </div>
                          <button
                            type="button"
                            onClick={() => revokeInvitationMutation.mutate(invite.id)}
                            disabled={!csrfToken || revokeInvitationMutation.isPending}
                            className="inline-flex h-9 items-center justify-center rounded-xl border border-white/10 bg-white/5 px-3 text-xs text-slate-200 transition hover:bg-white/10 disabled:cursor-not-allowed disabled:opacity-50"
                          >
                            Revoke
                          </button>
                        </div>
                      </div>
                    ))
                  ) : (
                    <p className="text-sm text-slate-400">No pending workspace invites.</p>
                  )}
                </div>
              </div>
            </section>

            <section className="rounded-3xl border border-white/10 bg-slate-900/70 p-6 backdrop-blur-xl">
              <p className="text-xs uppercase tracking-[0.2em] text-cyan-300/80">Current members</p>
              <div className="mt-4 grid gap-3">
                {members.length > 0 ? (
                  members.map((member) => (
                    <div key={member.user_id} className="rounded-2xl border border-white/10 bg-slate-950/55 px-4 py-4">
                      <div className="flex flex-col gap-3 lg:flex-row lg:items-center lg:justify-between">
                        <div className="min-w-0">
                          <p className="flex items-center gap-2 text-sm font-medium text-white">
                            <UserRound className="h-4 w-4 text-cyan-300" />
                            <span className="truncate">{member.display_name}</span>
                          </p>
                          <p className="mt-1 break-all text-xs text-slate-400">{member.email}</p>
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
                            className="h-10 rounded-xl border border-white/15 bg-slate-950/70 px-3 text-xs uppercase tracking-[0.12em] text-slate-100 outline-none ring-cyan-400 transition focus:ring-2"
                          >
                            <option value="admin">Admin</option>
                            <option value="writer">Writer</option>
                            <option value="reader">Reader</option>
                          </select>
                          <button
                            type="button"
                            onClick={() => removeMemberMutation.mutate(member.user_id)}
                            disabled={!csrfToken || removeMemberMutation.isPending}
                            className="inline-flex h-10 items-center gap-2 rounded-xl border border-rose-400/30 bg-rose-500/10 px-3 text-xs uppercase tracking-[0.12em] text-rose-100 transition hover:bg-rose-500/20 disabled:cursor-not-allowed disabled:opacity-50"
                          >
                            <UserX className="h-3.5 w-3.5" />
                            Remove
                          </button>
                        </div>
                      </div>
                    </div>
                  ))
                ) : (
                  <p className="text-sm text-slate-400">No active members yet.</p>
                )}
              </div>
            </section>
          </>
        ) : null}
      </main>
    </div>
  );
}
