# Alpha-Lago Adapter Plan

This document turns Phase 1 of the roadmap into a concrete refactor plan.

Use it to move Alpha toward a clean adapter boundary without doing a large risky rewrite.

## Goal

Stabilize the Lago boundary inside Alpha so that:
- product services and handlers stop depending on `LagoClient` directly
- Lago calls are grouped by domain responsibility
- future onboarding, customer, and payment work can build on stable Alpha-side interfaces

This is a code-structure refactor, not a user-visible product redesign.

## Current Direct Lago Coupling

Today the main Lago coupling lives in these areas.

### 1. API handler coupling
Current direct handler usage is in:
- [http.go](/Users/superuser/projects/golang/usage-billing-control-plane/internal/api/http.go)

Current direct Lago calls there:
- `SyncMeter(...)`
- `ProxyInvoicePreview(...)`
- `ProxyInvoiceRetryPayment(...)`
- `ProxyInvoiceByID(...)`

This is the first thing to clean up.
Handlers should depend on Alpha-side services or interfaces, not on `LagoClient`.

### 2. Transport client coupling
Current transport client:
- [lago_client.go](/Users/superuser/projects/golang/usage-billing-control-plane/internal/service/lago_client.go)

It currently mixes multiple concerns in one client:
- meter sync
- invoice preview pass-through
- invoice retry-payment pass-through
- invoice fetch by id
- generic raw request helper

This is workable, but it is too broad as a long-term dependency surface.

### 3. Webhook verification coupling
Current webhook verifier and tenant routing logic:
- [lago_webhook_service.go](/Users/superuser/projects/golang/usage-billing-control-plane/internal/service/lago_webhook_service.go)

This file currently combines:
- webhook signature verification
- public key retrieval from Lago
- tenant mapping from `organization_id`
- webhook projection handling

This should be split by responsibility, but in small steps.

### 4. Payment reconciliation coupling
Current payment reconcile service:
- [payment_reconcile_service.go](/Users/superuser/projects/golang/usage-billing-control-plane/internal/service/payment_reconcile_service.go)

It currently depends on `LagoClient` for invoice fetch.
That is acceptable for now, but it should depend on an invoice-focused adapter interface instead.

### 5. Server wiring coupling
Current server composition:
- [main.go](/Users/superuser/projects/golang/usage-billing-control-plane/cmd/server/main.go)

Current wiring creates one `LagoClient` and passes it into API and webhook code directly.
That should evolve into domain adapters composed at startup and injected by responsibility.

## Recommended Target Boundary

Do not jump to many interfaces immediately.
Use a minimal set that matches current behavior.

### A. Meter sync adapter
Responsibility:
- sync Alpha meter intent into Lago billable metrics

Suggested interface:
```go
type MeterSyncAdapter interface {
    SyncMeter(ctx context.Context, meter domain.Meter) error
}
```

### B. Invoice adapter
Responsibility:
- invoice preview
- invoice fetch
- retry payment

Suggested interface:
```go
type InvoiceBillingAdapter interface {
    PreviewInvoice(ctx context.Context, payload []byte) (int, []byte, error)
    GetInvoice(ctx context.Context, invoiceID string) (int, []byte, error)
    RetryInvoicePayment(ctx context.Context, invoiceID string, payload []byte) (int, []byte, error)
}
```

### C. Webhook key provider / verifier boundary
Responsibility:
- fetch webhook public key from Lago
- verify webhook signature

Suggested split:
```go
type WebhookKeyProvider interface {
    FetchWebhookPublicKey(ctx context.Context) (*rsa.PublicKey, error)
}

type WebhookVerifier interface {
    Verify(ctx context.Context, headers http.Header, body []byte) error
}
```

### D. Tenant billing mapper
Responsibility:
- resolve Alpha tenant from Lago organization id

Current tenant-backed mapper is conceptually correct.
Keep that boundary.

## Minimal Refactor Sequence

Do this in slices.
Do not do a big-bang rewrite.

## Slice 1: Stop API handlers from depending on `LagoClient`

Introduce small Alpha-side adapter interfaces and inject those into the server.

Change:
- handlers in [http.go](/Users/superuser/projects/golang/usage-billing-control-plane/internal/api/http.go)

So that handlers call:
- `meterSyncAdapter`
- `invoiceBillingAdapter`

Instead of:
- `s.lagoClient`

Why first:
- this is the highest-value cleanup
- it reduces the direct transport dependency at the API edge
- it makes later customer/payment flows cleaner

## Slice 2: Split `LagoClient` into domain adapters without changing behavior

Keep the HTTP implementation if you want, but stop presenting it as one broad client.

Recommended concrete types:
- `LagoMeterSyncAdapter`
- `LagoInvoiceAdapter`
- `LagoWebhookKeyProvider`

These can still share a small internal HTTP helper.

Do not change endpoints or product behavior in this slice.
Just narrow the dependency surface.

## Slice 3: Narrow webhook responsibilities

Refactor [lago_webhook_service.go](/Users/superuser/projects/golang/usage-billing-control-plane/internal/service/lago_webhook_service.go) into cleaner pieces:
- webhook verifier
- key provider
- tenant mapper
- webhook projector/service

This does not need to change the webhook endpoint.
It is mostly internal cleanup.

## Slice 4: Switch reconcile and explainability paths to invoice-focused adapter use

Update:
- [payment_reconcile_service.go](/Users/superuser/projects/golang/usage-billing-control-plane/internal/service/payment_reconcile_service.go)
- invoice-related HTTP paths and services

So that they depend on `InvoiceBillingAdapter`, not `LagoClient`.

This makes invoice/payment execution access look like one billing boundary rather than random direct calls.

## Slice 5: Update server composition to wire adapters by responsibility

In [main.go](/Users/superuser/projects/golang/usage-billing-control-plane/cmd/server/main.go), compose:
- invoice adapter
- meter sync adapter
- webhook verifier
- tenant mapper

Then inject those into:
- server
- webhook service
- reconcile services

That is the point where the architectural boundary becomes obvious in startup wiring.

## What To Leave Alone In Phase 1

Do not try to solve these yet in the same refactor:
- customer model design
- payment method product model
- onboarding redesign
- self-signup
- replacing all Lago terminology in docs or payloads

That would mix structural cleanup with product work.
Keep Phase 1 narrow.

## Concrete File Mapping

### Current direct Lago edge
- [http.go](/Users/superuser/projects/golang/usage-billing-control-plane/internal/api/http.go)
- [main.go](/Users/superuser/projects/golang/usage-billing-control-plane/cmd/server/main.go)

### Current transport implementation
- [lago_client.go](/Users/superuser/projects/golang/usage-billing-control-plane/internal/service/lago_client.go)

### Current webhook boundary
- [lago_webhook_service.go](/Users/superuser/projects/golang/usage-billing-control-plane/internal/service/lago_webhook_service.go)

### Current invoice/payment consumers
- [payment_reconcile_service.go](/Users/superuser/projects/golang/usage-billing-control-plane/internal/service/payment_reconcile_service.go)
- [invoice_explainability_service.go](/Users/superuser/projects/golang/usage-billing-control-plane/internal/service/invoice_explainability_service.go)

## Definition Of Done For Phase 1

Phase 1 is done when:
- API handlers no longer call `LagoClient` directly
- `LagoClient` is no longer the broad dependency type passed through the app
- Lago access is grouped into small domain adapters
- webhook verification and invoice access each have explicit boundaries
- startup wiring shows Lago dependencies by responsibility, not as one generic client

## Recommended Next Implementation Step

Start with Slice 1 only:
- add small invoice and meter adapter interfaces
- update server wiring
- keep underlying behavior and tests the same

That gives a clean first refactor without dragging customer or payment-model work into the same change.
