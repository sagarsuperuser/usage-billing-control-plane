
import { useState } from "react";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import {
  Copy,
  LoaderCircle,
  MailPlus,
  UserX,
  X,
} from "lucide-react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import { StatusChip } from "@/components/ui/status-chip";
import { MiniPagination } from "@/components/ui/mini-pagination";

import {
  createTenantWorkspaceInvitation,
  fetchTenantWorkspaceInvitations,
  fetchTenantWorkspaceMembers,
  removeTenantWorkspaceMember,
  revokeTenantWorkspaceInvitation,
  updateTenantWorkspaceMember,
} from "@/lib/api";
import { formatRelativeTimestamp } from "@/lib/format";
import { showError, showSuccess } from "@/lib/toast";

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
      createTenantWorkspaceInvitation({ runtimeBaseURL: apiBaseURL, csrfToken, email: data.email, role: data.role }),
    onSuccess: async () => {
      resetInvite();
      await queryClient.invalidateQueries({ queryKey: invitationQueryKey });
    },
    onError: (err: Error) => showError(err.message),
  });
  const revokeInvitationMutation = useMutation({
    mutationFn: (invitationID: string) => revokeTenantWorkspaceInvitation({ runtimeBaseURL: apiBaseURL, csrfToken, invitationID }),
    onSuccess: async () => { showSuccess("Invitation revoked"); await queryClient.invalidateQueries({ queryKey: invitationQueryKey }); },
    onError: (err: Error) => showError(err.message),
  });
  const updateMemberMutation = useMutation({
    mutationFn: (input: { userID: string; role: "reader" | "writer" | "admin" }) =>
      updateTenantWorkspaceMember({ runtimeBaseURL: apiBaseURL, csrfToken, userID: input.userID, role: input.role }),
    onSuccess: async (_payload, input) => {
      showSuccess("Role updated");
      setMemberDraftRoles((c) => { const n = { ...c }; delete n[input.userID]; return n; });
      setConfirmingMemberAction(null);
      setSelectedMemberID("");
      await queryClient.invalidateQueries({ queryKey: workspaceQueryKey });
    },
    onError: (err: Error) => showError(err.message),
  });
  const removeMemberMutation = useMutation({
    mutationFn: (userID: string) => removeTenantWorkspaceMember({ runtimeBaseURL: apiBaseURL, csrfToken, userID }),
    onSuccess: async () => {
      showSuccess("Member removed");
      setConfirmingMemberAction(null);
      setSelectedMemberID("");
      await queryClient.invalidateQueries({ queryKey: workspaceQueryKey });
    },
    onError: (err: Error) => showError(err.message),
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

  const pagedMembers = paginateItems(members, memberPage, 10);
  const pagedInvitations = paginateItems(pendingInvitations, invitePage, 5);

  const selectedMember = members.find((m) => m.user_id === selectedMemberID) ?? null;

  /* --- Render ----------------------------------------------------- */

  return (
    <>
      {/* Invite modal */}
      {showInviteModal && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4" onClick={(e) => { if (e.target === e.currentTarget) { setShowInviteModal(false); createInvitationMutation.reset(); } }}>
          <div className="w-full max-w-md rounded-xl bg-surface shadow-2xl ring-1 ring-black/10">
            <div className="flex items-center justify-between border-b border-border px-6 py-4">
              <p className="font-semibold text-text-primary">Invite member</p>
              <button type="button" onClick={() => { setShowInviteModal(false); createInvitationMutation.reset(); }} className="inline-flex h-7 w-7 items-center justify-center rounded-lg text-text-faint transition hover:bg-surface-tertiary hover:text-text-secondary">
                <X className="h-4 w-4" />
              </button>
            </div>
            <div className="p-6">
              {latestInviteURL ? (
                <div className="rounded-lg border border-emerald-200 bg-emerald-50 px-4 py-4">
                  <p className="text-sm font-semibold text-emerald-800">Invitation sent</p>
                  <p className="mt-2 break-all rounded border border-emerald-100 bg-surface px-3 py-2 font-mono text-xs text-text-secondary">{latestInviteURL}</p>
                  <button type="button" onClick={() => { void navigator.clipboard.writeText(latestInviteURL); }} className="mt-3 inline-flex h-7 items-center gap-1.5 rounded border border-border bg-surface px-2.5 text-xs text-text-secondary transition hover:bg-surface-tertiary">
                    <Copy className="h-3 w-3" />
                    Copy link
                  </button>
                </div>
              ) : (
                <div className="grid gap-3">
                  <div>
                    <label className="mb-1.5 block text-xs font-medium text-text-secondary">Email</label>
                    <input {...registerInvite("email")} type="email" placeholder="teammate@example.com" className="h-9 w-full rounded-lg border border-border bg-surface px-3 text-sm text-text-primary outline-none ring-slate-400 transition focus:ring-2" />
                  </div>
                  <div>
                    <label className="mb-1.5 block text-xs font-medium text-text-secondary">Role</label>
                    <select {...registerInvite("role")} aria-label="Workspace role" className="h-9 w-full rounded-lg border border-border bg-surface px-3 text-sm text-text-primary outline-none ring-slate-400 transition focus:ring-2">
                      <option value="admin">Admin</option>
                      <option value="writer">Writer</option>
                      <option value="reader">Reader</option>
                    </select>
                  </div>
                  <button
                    type="button"
                    onClick={handleInviteSubmit((data) => createInvitationMutation.mutate(data))}
                    disabled={!csrfToken || createInvitationMutation.isPending}
                    className="inline-flex h-9 items-center justify-center gap-2 rounded-lg bg-slate-900 px-4 text-sm font-medium text-white transition hover:bg-slate-800 disabled:opacity-50"
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
          <div className="h-full w-full max-w-sm border-l border-border bg-surface shadow-xl overflow-y-auto">
            <div className="flex items-center justify-between border-b border-border px-5 py-3.5">
              <div className="min-w-0">
                <p className="font-semibold text-text-primary">{selectedMember.display_name}</p>
                <p className="mt-0.5 truncate text-xs text-text-muted">{selectedMember.email}</p>
              </div>
              <div className="flex items-center gap-2">
                <StatusChip tone={selectedMember.status === "active" ? "success" : "neutral"}>{selectedMember.status}</StatusChip>
                <button type="button" onClick={() => setSelectedMemberID("")} className="inline-flex h-6 w-6 items-center justify-center rounded text-text-faint transition hover:bg-surface-tertiary hover:text-text-secondary">
                  <X className="h-3.5 w-3.5" />
                </button>
              </div>
            </div>
            <div className="p-5">
              {(() => {
                const draftRole = memberDraftRoles[selectedMember.user_id] ?? (selectedMember.role as "reader" | "writer" | "admin");
                const roleDirty = draftRole !== selectedMember.role;
                const selfMember = isSelfMember(selectedMember.user_id);
                const lastAdminProtected = isLastActiveAdmin(selectedMember);
                const roleSelectDisabled = selectedMember.status !== "active" || updateMemberMutation.isPending || selfMember || lastAdminProtected;
                const canApplyRole = roleDirty && !roleSelectDisabled && Boolean(csrfToken);
                const canSuspend = selectedMember.status === "active" && Boolean(csrfToken) && !removeMemberMutation.isPending && !selfMember && !lastAdminProtected;
                const canReactivate = selectedMember.status !== "active" && Boolean(csrfToken) && !updateMemberMutation.isPending && !selfMember;
                const showSuspendConfirm = confirmingMemberAction?.userID === selectedMember.user_id && confirmingMemberAction.action === "suspend";
                return (
                  <div className="grid gap-4">
                    <div>
                      <label className="mb-1.5 block text-xs font-medium text-text-muted">Role</label>
                      <select
                        aria-label={`Role for ${selectedMember.email}`}
                        value={draftRole}
                        onChange={(event) => setMemberDraftRoles((c) => ({ ...c, [selectedMember.user_id]: event.target.value as "reader" | "writer" | "admin" }))}
                        disabled={roleSelectDisabled}
                        className="h-9 w-full rounded-lg border border-border bg-surface px-3 text-sm text-text-primary outline-none ring-slate-400 transition focus:ring-2 disabled:opacity-50"
                      >
                        <option value="admin">Admin</option>
                        <option value="writer">Writer</option>
                        <option value="reader">Reader</option>
                      </select>
                    </div>

                    {roleDirty ? (
                      <div className="flex gap-2">
                        <button type="button" onClick={() => updateMemberMutation.mutate({ userID: selectedMember.user_id, role: draftRole })} disabled={!canApplyRole} className="flex-1 inline-flex h-8 items-center justify-center rounded-lg bg-slate-900 text-sm font-medium text-white transition hover:bg-slate-800 disabled:opacity-50">Save</button>
                        <button type="button" onClick={() => setMemberDraftRoles((c) => { const n = { ...c }; delete n[selectedMember.user_id]; return n; })} className="flex-1 inline-flex h-8 items-center justify-center rounded-lg border border-border text-sm text-text-secondary hover:bg-surface-tertiary">Cancel</button>
                      </div>
                    ) : selectedMember.status === "active" ? (
                      showSuspendConfirm ? (
                        <div className="flex gap-2">
                          <button type="button" onClick={() => removeMemberMutation.mutate(selectedMember.user_id)} disabled={!canSuspend} className="flex-1 inline-flex h-8 items-center justify-center rounded-lg bg-rose-600 text-sm font-medium text-white hover:bg-rose-700 disabled:opacity-50">Confirm</button>
                          <button type="button" onClick={() => setConfirmingMemberAction(null)} className="flex-1 inline-flex h-8 items-center justify-center rounded-lg border border-border text-sm text-text-secondary hover:bg-surface-tertiary">Cancel</button>
                        </div>
                      ) : (
                        <button type="button" onClick={() => setConfirmingMemberAction({ userID: selectedMember.user_id, action: "suspend" })} disabled={!canSuspend} className="inline-flex h-8 w-full items-center justify-center gap-2 rounded-lg border border-rose-200 text-sm font-medium text-rose-600 hover:bg-rose-50 disabled:opacity-50">
                          <UserX className="h-3.5 w-3.5" />
                          Suspend member
                        </button>
                      )
                    ) : (
                      <button type="button" onClick={() => updateMemberMutation.mutate({ userID: selectedMember.user_id, role: selectedMember.role as "reader" | "writer" | "admin" })} disabled={!canReactivate} className="inline-flex h-8 w-full items-center justify-center rounded-lg bg-slate-900 text-sm font-medium text-white hover:bg-slate-800 disabled:opacity-50">Reactivate</button>
                    )}

                    {selfMember ? <p className="text-xs text-text-faint">You cannot change your own membership.</p> : null}
                    {lastAdminProtected ? <p className="text-xs text-text-faint">Promote another admin first.</p> : null}
                  </div>
                );
              })()}
            </div>
          </div>
        </div>
      )}

      {/* Content */}
      <div className="divide-y divide-border">
        {/* Members section */}
        <div>
          <div className="flex items-center justify-between px-5 py-3">
            <div className="flex items-center gap-3">
              <p className="text-sm font-semibold text-text-primary">Members ({members.length})</p>
              <MiniPagination page={pagedMembers.page} totalPages={pagedMembers.totalPages} onPageChange={setMemberPage} label="Members" />
            </div>
            <button
              type="button"
              onClick={() => { resetInvite(); createInvitationMutation.reset(); setShowInviteModal(true); }}
              disabled={!csrfToken}
              className="inline-flex h-7 items-center gap-1 rounded bg-slate-900 px-2.5 text-xs font-medium text-white transition hover:bg-slate-800 disabled:opacity-50"
            >
              <MailPlus className="h-3 w-3" />
              Invite
            </button>
          </div>
          {pagedMembers.items.length > 0 ? (
            <table className="w-full text-sm">
              <thead>
                <tr className="border-y border-border-light text-left text-[11px] font-medium uppercase tracking-wider text-text-faint">
                  <th className="px-5 py-2 font-semibold">Member</th>
                  <th className="px-4 py-2 font-semibold">Role</th>
                  <th className="px-4 py-2 font-semibold">Status</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-border-light">
                {pagedMembers.items.map((member) => (
                  <tr
                    key={member.user_id}
                    onClick={() => setSelectedMemberID(member.user_id)}
                    className={`cursor-pointer transition ${selectedMemberID === member.user_id ? "bg-sky-50" : "hover:bg-surface-secondary"}`}
                  >
                    <td className="px-5 py-2.5">
                      <p className="font-medium text-text-primary">{member.display_name}</p>
                      <p className="text-xs text-text-faint">{member.email}</p>
                    </td>
                    <td className="px-4 py-2.5 text-xs capitalize text-text-muted">{member.role}</td>
                    <td className="px-4 py-2.5">
                      <div className="flex flex-wrap gap-1">
                        <StatusChip tone={member.status === "active" ? "success" : "neutral"}>{member.status}</StatusChip>
                        {isSelfMember(member.user_id) ? <StatusChip tone="info">You</StatusChip> : null}
                        {isLastActiveAdmin(member) ? <StatusChip tone="warning">Last admin</StatusChip> : null}
                      </div>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          ) : (
            <p className="px-5 py-6 text-xs text-text-faint">No members yet.</p>
          )}
        </div>

        {/* Pending invites section */}
        <div>
          <div className="flex items-center justify-between px-5 py-3">
            <div className="flex items-center gap-3">
              <p className="text-sm font-medium text-text-secondary">Pending invites ({pendingInvitations.length})</p>
              <MiniPagination page={pagedInvitations.page} totalPages={pagedInvitations.totalPages} onPageChange={setInvitePage} label="Pending invites" />
            </div>
          </div>
          {pagedInvitations.items.length > 0 ? (
            <table className="w-full text-sm">
              <thead>
                <tr className="border-y border-border-light text-left text-[11px] font-medium uppercase tracking-wider text-text-faint">
                  <th className="px-5 py-2 font-semibold">Email</th>
                  <th className="px-4 py-2 font-semibold">Role</th>
                  <th className="px-4 py-2 font-semibold">Expires</th>
                  <th className="px-4 py-2 font-semibold" />
                </tr>
              </thead>
              <tbody className="divide-y divide-border-light">
                {pagedInvitations.items.map((invite) => (
                  <tr key={invite.id}>
                    <td className="px-5 py-2.5 font-medium text-text-primary">{invite.email}</td>
                    <td className="px-4 py-2.5 text-xs capitalize text-text-muted">{invite.role}</td>
                    <td className="px-4 py-2.5 text-xs text-text-faint">{formatRelativeTimestamp(invite.expires_at)}</td>
                    <td className="px-4 py-2.5 text-right">
                      <button type="button" onClick={() => revokeInvitationMutation.mutate(invite.id)} disabled={!csrfToken || revokeInvitationMutation.isPending} className="text-xs font-medium text-rose-600 hover:text-rose-800 disabled:opacity-50">
                        Revoke
                      </button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          ) : (
            <p className="px-5 py-4 text-xs text-text-faint">No pending invites.</p>
          )}
        </div>
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
