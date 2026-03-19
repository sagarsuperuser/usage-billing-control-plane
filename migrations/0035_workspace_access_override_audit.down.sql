DELETE FROM tenant_audit_events
WHERE action IN (
  'workspace_member_role_changed',
  'workspace_member_disabled',
  'workspace_member_reactivated',
  'workspace_invitation_revoked'
);

ALTER TABLE tenant_audit_events
  DROP CONSTRAINT IF EXISTS chk_tenant_audit_events_action_allowed;

ALTER TABLE tenant_audit_events
  ADD CONSTRAINT chk_tenant_audit_events_action_allowed
  CHECK (
    action IN (
      'created',
      'status_changed',
      'updated',
      'payment_setup_requested',
      'payment_setup_resent'
    )
  );
