ALTER TABLE invoice_dunning_events
  DROP CONSTRAINT IF EXISTS chk_invoice_dunning_events_event_type;

ALTER TABLE invoice_dunning_events
  ADD CONSTRAINT chk_invoice_dunning_events_event_type
    CHECK (event_type IN (
      'dunning_started',
      'retry_scheduled',
      'payment_setup_pending',
      'payment_setup_ready',
      'paused',
      'resumed',
      'escalated',
      'resolved'
    ));
