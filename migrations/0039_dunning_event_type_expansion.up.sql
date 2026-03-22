ALTER TABLE invoice_dunning_events
  DROP CONSTRAINT IF EXISTS chk_invoice_dunning_events_event_type;

ALTER TABLE invoice_dunning_events
  ADD CONSTRAINT chk_invoice_dunning_events_event_type
    CHECK (event_type IN (
      'dunning_started',
      'retry_scheduled',
      'retry_attempted',
      'retry_succeeded',
      'retry_failed',
      'payment_setup_pending',
      'payment_setup_ready',
      'notification_sent',
      'notification_failed',
      'paused',
      'resumed',
      'escalated',
      'resolved'
    ));
