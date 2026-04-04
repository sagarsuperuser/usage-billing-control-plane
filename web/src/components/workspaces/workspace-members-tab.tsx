"use client";

import { type ReactNode, useState } from "react";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import {
  ChevronLeft,
  ChevronRight,
  Copy,
  LoaderCircle,
  MailPlus,
  UserX,
  X,
} from "lucide-react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import {
  createTenantWorkspaceInvitation,
  fetchTenantWorkspaceInvitations,
  fetchTenantWorkspaceMembers,
  removeTenantWorkspaceMember,
  revokeTenantWorkspaceInvitation,
  updateTenantWorkspaceMember,
} from "@/lib/api";
import { formatExactTimestamp } from "@/lib/format";

/* ------------------------------------------------------------------ */
/*  Types                                                              */
/* ------------------------------------------------------------------ */

export interface WorkspaceMembersTabProps {
  apiBaseURL: string;
  csrfToken: string;
  isAdmin: boolean;
  session: { tenant_id?: string; subject_id?: string } | null;
}

/* ------------------------------------------------------------------ */
/*  Component                                                          */
/* ------------------------------------------------------------------ */

export function WorkspaceMembersTab({ apiBaseURL, csrfToken, session }: WorkspaceMembersTabProps) {
  const queryClient = useQueryClient();

  const { register: registerInvite, handleSubmit: handleInviteSubmit, reset: resetInvite } = useForm({
    resolver: zodResolver(z.object({ email: z.string().email(), role: z.enum(["reader", "writer", "admin"]) })),
    defaultValues: { email: "", role: "writer" as const },
  });

  const [selectedMemberID, setSelectedMemberID] = useState("");
  const [memberDraftRoles, setMemberDraftRoles] = useState<Record<string, "reader" | "writer" | "admin">>({});
  const [confirmingMemberAction, setConfirmingMemberAction] = useState<{ userID: string; action: "suspend" } | null>(null);
  const [memberPage, setMemberPage] = useState(1);
  const [invitePage, setInvitePage] = useState(1);
  const [showInviteModal, setShowInviteModal] = useState(false);

  /* --- Queries ---------------------------------------------------- */

  const workspaceQueryKey = ["tenant-workspace-members", apiBaseURL, session?.tenant_id];
  const invitationQueryKey = ["tenant-workspace-invitations", apiBaseURL, session?.tenant_id];

  const membersQuery = useQuery({
    queryKey: workspaceQueryKey,
    queryFn: () => fetchTenantWorkspaceMembers({ runtimeBaseURL: apiBaseURL }),
    enabled: Boolean(session),
  });
  const invitationsQuery = useQuery({
    queryKey: invitationQueryKey,
    queryFn: () => fetchTenantWorkspaceInvitations({ runtimeBaseURL: apiBaseURL }),
    enabled: Boolean(session),
  });

  /* --- Mutations -------------------------------------------------- */

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
      setSelectedMemberID("");
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
      setSelectedMemberID("");
      await queryClient.invalidateQueries({ queryKey: workspaceQueryKey });
    },
  });

  /* --- Derived ---------------------------------------------------- */

  const members = membersQuery.data ?? [];
  const invitations = invitationsQuery.data ?? [];
  const pendingInvitations = invitations.filter((item) => item.status === "pending");
  const latestInviteURL = createInvitationMutation.data?.accept_url ?? "";
  const currentUserID = session?.subject_id ?? "";
  const activeAdminCount = members.filter((m) => m.status === "active" && m.role === "admin").length;

  const isSelfMember = (userID: string): boolean => currentUserID !== "" && currentUserID === userID;
  const isLastActiveAdmin = (member: { role: string; status: string }): boolean =>
    member.status === "active" && member.role === "admin" && activeAdminCount <= 1;

  const pagedMembers = paginateItems(members, memberPage, 8);
  const pagedInvitations = paginateItems(pendingInvitations, invitePage, 5);

  const selectedMemberIDValue = selectedMemberID || "";
  const selectedMember = members.find((m) => m.user_id === selectedMemberIDValue) ?? null;

  /* --- Render ----------------------------------------------------- */

  return (
    <div className="p-6">
      {/* Invite modal */}
      {showInviteModal && (
        <div
          className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4"
          onClick={(e) => { if (e.target === e.currentTarget) { setShowInviteModal(false); createInvitationMutation.reset(); } }}
        >
          <div className="w-full max-w-md rounded-xl bg-white shadow-2xl ring-1 ring-black/10">
            <div className="flex items-center justify-between border-b border-stone-200 px-6 py-4">
              <div>
                <p className="font-semibold text-slate-900">Invite a member</p>
                <p className="mt-0.5 text-xs text-slate-500">Send a workspace invitation by email.</p>
              </div>
              <button
                type="button"
                onClick={() => { setShowInviteModal(false); createInvitationMutation.reset(); }}
                className="inline-flex h-8 w-8 items-center justify-center rounded-lg border border-stone-200 text-slate-400 transition hover:bg-stone-100 hover:text-slate-700"
              >
                <X className="h-4 w-4" />
              </button>
            </div>
            <div className="p-6">
              {latestInviteURL ? (
                <div className="rounded-lg border border-emerald-200 bg-emerald-50 px-4 py-4">
                  <p className="text-sm font-semibold text-emerald-800">Invitation sent</p>
                  <p className="mt-1 text-xs text-slate-600">Share this link with the invitee:</p>
                  <p className="mt-2 break-all rounded border border-emerald-100 bg-white px-3 py-2 font-mono text-xs text-slate-800">{latestInviteURL}</p>
                  <button
                    type="button"
                    onClick={() => { void navigator.clipboard.writeText(latestInviteURL); }}
                    className="mt-3 inline-flex h-8 items-center gap-1.5 rounded-lg border border-stone-200 bg-white px-3 text-xs text-slate-700 transition hover:bg-stone-100"
                  >
                    <Copy className="h-3 w-3" />
                    Copy link
                  </button>
                </div>
              ) : (
                <div className="grid gap-3">
                  <div>
                    <label className="mb-1.5 block text-xs font-medium text-slate-700">Email</label>
                    <input
                      {...registerInvite("email")}
                      type="email"
                      placeholder="teammate@example.com"
                      className="h-10 w-full rounded-lg border border-stone-200 bg-white px-3 text-sm text-slate-900 outline-none ring-slate-400 transition focus:ring-2"
                    />
                  </div>
                  <div>
                    <label className="mb-1.5 block text-xs font-medium text-slate-700">Role</label>
                    <select
                      {...registerInvite("role")}
                      aria-label="Workspace role"
                      className="h-10 w-full rounded-lg border border-stone-200 bg-white px-3 text-sm text-slate-900 outline-none ring-slate-400 transition focus:ring-2"
                    >
                      <option value="admin">Admin</option>
                      <option value="writer">Writer</option>
                      <option value="reader">Reader</option>
                    </select>
                  </div>
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
              )}
            </div>
          </div>
        </div>
      )}

      {/* Member slide-out panel */}
      {selectedMember && (
        <div className="fixed inset-0 z-40 flex justify-end" onClick={(e) => { if (e.target === e.currentTarget) setSelectedMemberID(""); }}>
          <div className="h-full w-full max-w-md border-l border-stone-200 bg-white shadow-xl overflow-y-auto">
            <div className="flex items-center justify-between border-b border-stone-200 px-6 py-4">
              <div className="min-w-0">
                <p className="font-semibold text-slate-900">{selectedMember.display_name}</p>
                <p className="mt-0.5 truncate text-xs text-slate-500">{selectedMember.email}</p>
              </div>
              <div className="flex items-center gap-2">
                <StatusChip tone={selectedMember.status === "active" ? "success" : "neutral"}>{selectedMember.status}</StatusChip>
                <button
                  type="button"
                  onClick={() => setSelectedMemberID("")}
                  className="inline-flex h-8 w-8 items-center justify-center rounded-lg border border-stone-200 text-slate-400 transition hover:bg-stone-100 hover:text-slate-700"
                >
                  <X className="h-4 w-4" />
                </button>
              </div>
            </div>
            <div className="p-6">
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
                  <div className="grid gap-4">
                    <dl className="grid gap-3">
                      <div>
                        <dt className="text-xs font-medium text-slate-500">Name</dt>
                        <dd className="mt-1 text-sm text-slate-900">{selectedMember.display_name}</dd>
                      </div>
                      <div>
                        <dt className="text-xs font-medium text-slate-500">Email</dt>
                        <dd className="mt-1 text-sm text-slate-900">{selectedMember.email}</dd>
                      </div>
                    </dl>

                    <div>
                      <label className="mb-1.5 block text-xs font-medium text-slate-500">Role</label>
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
                        className="h-10 w-full rounded-lg border border-stone-200 bg-white px-3 text-sm text-slate-900 outline-none ring-slate-400 transition focus:ring-2"
                      >
                        <option value="admin">Admin</option>
                        <option value="writer">Writer</option>
                        <option value="reader">Reader</option>
                      </select>
                    </div>

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

                    {selfMember ? <p className="text-xs text-slate-500">You cannot change your own membership from this screen.</p> : null}
                    {lastAdminProtected ? <p className="text-xs text-slate-500">Promote another active admin before changing this member&apos;s access.</p> : null}
                  </div>
                );
              })()}
            </div>
          </div>
        </div>
      )}

      {/* Header with count + invite button */}
      <div className="flex items-center justify-between">
        <p className="text-sm font-semibold text-slate-900">Members ({members.length})</p>
        <button
          type="button"
          onClick={() => { resetInvite(); createInvitationMutation.reset(); setShowInviteModal(true); }}
          disabled={!csrfToken}
          className="inline-flex h-8 items-center gap-1.5 rounded-lg border border-slate-900 bg-slate-900 px-3 text-xs font-medium text-white transition hover:bg-slate-800 disabled:cursor-not-allowed disabled:opacity-50"
        >
          <MailPlus className="h-3.5 w-3.5" />
          Invite
        </button>
      </div>

      {/* Members table — full width */}
      <div className="mt-4 overflow-hidden rounded-lg border border-stone-200">
        <div className="flex items-center justify-between border-b border-stone-200 bg-stone-50 px-4 py-3">
          <p className="text-sm font-medium text-slate-700">Active members</p>
          <PaginationControls page={pagedMembers.page} totalPages={pagedMembers.totalPages} onPageChange={setMemberPage} label="Members" />
        </div>
        {pagedMembers.items.length > 0 ? (
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b border-stone-100 text-left text-xs font-medium text-slate-400">
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

      {/* Pending invites — compact section below */}
      <div className="mt-4 border-t border-stone-200 pt-4">
        <div className="flex items-center justify-between">
          <p className="text-sm font-medium text-slate-700">Pending invites ({pendingInvitations.length})</p>
          <PaginationControls page={pagedInvitations.page} totalPages={pagedInvitations.totalPages} onPageChange={setInvitePage} label="Pending invites" />
        </div>
        {pagedInvitations.items.length > 0 ? (
          <table className="mt-2 w-full text-sm">
            <thead>
              <tr className="border-b border-stone-100 text-left text-xs font-medium text-slate-400">
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
          <p className="mt-2 text-sm text-slate-500">No pending workspace invites.</p>
        )}
      </div>
    </div>
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
