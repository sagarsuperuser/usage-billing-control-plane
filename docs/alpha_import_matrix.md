# Alpha Import Matrix

This document turns the Alpha import goal into a working execution matrix.

Use it as the tracking layer for deciding:

- what should be imported into Alpha
- what should be redesigned instead of copied
- what backend work is required
- what UI surfaces are required
- what should be built first

This document should be read together with:

- [Alpha Import Goal](./alpha_import_goal.md)
- [Alpha Wave 1 Roadmap](./alpha_wave1_roadmap.md)

---

## Market Signal Lens

Use this lens when deciding whether a surface is:

- essential to compete credibly
- necessary for enterprise maturity
- useful later, but not core to the first strong Alpha product

### 1. Market table stakes

These are the capabilities a serious billing control plane is generally expected to cover.

- human browser auth
- SSO foundation
- customers
- pricing and catalog
- subscriptions
- invoices
- payments
- taxes
- billing connections
- basic admin controls

If Alpha lacks these, it will feel strategically incomplete relative to mature billing products.

### 2. Enterprise maturity

These are the capabilities that make the product operationally credible for larger or more regulated customers.

- invitations and team management
- roles and permissions
- auth-provider admin settings
- credit notes
- dunning
- billing entities
- invoice configuration
- auditability and developer controls
- broader provider/integration management

If Alpha lacks these, it may still work, but it will feel less enterprise-ready.

### 3. Later-stage completeness and differentiation

These are useful, but should not outrank core billing and admin coverage.

- richer analytics breadth
- forecasting
- customer portal
- very broad integration catalog
- advanced developer tooling beyond core needs

These are valuable once Alpha already feels complete as the primary control plane.

---

## Status Legend

- `Present`: already meaningfully present in Alpha
- `Partial`: foundation exists, but the product surface is incomplete
- `Missing`: not meaningfully present yet

## Priority Legend

- `P0`: must-have to make Alpha the true control plane
- `P1`: high-value follow-up needed for product completeness
- `P2`: important, but after core billing and admin coverage
- `P3`: later-stage enhancement

---

## Import Matrix

| Domain | Lago capability | Alpha state | Alpha-native shape | Backend needed | UI needed | Priority | Notes |
|---|---|---|---|---|---|---|---|
| Auth | Human browser login | Present | Keep Alpha-owned | Continue user/session hardening | Continue dedicated auth surface | P0 | Already the correct direction |
| Auth | SSO / OIDC extensibility | Partial | Alpha-owned SSO and auth settings | Provider config, provisioning, membership mapping | Auth settings UI | P0 | Required for enterprise-grade product |
| Auth | Signup / invite / reset flows | Missing | Alpha-owned user lifecycle | User invite, password reset, email flows | Signup/invite/reset screens | P1 | Needed if Alpha fully owns browser auth |
| Platform | Workspace directory | Present | Keep as `Workspaces` | Incremental enrichment only | Incremental polish only | P0 | Already aligned with Alpha IA |
| Platform | Workspace setup | Present | Keep as setup-only flow | Incremental polish only | Incremental polish only | P0 | Already aligned with Alpha IA |
| Platform | Workspace detail | Present | Keep as detail surface | Readiness enrichment over time | Detail enrichment over time | P0 | Good foundation |
| Platform | Billing connections | Partial | Keep as Alpha-owned `Billing Connections` | Expand sync/status/rotation/provider support | Improve list/detail/create flows | P0 | Strategic replacement for Lago provider plumbing |
| Tenant | Customer directory | Present | Keep as `Customers` | Incremental enrichment only | Incremental polish only | P0 | Good foundation |
| Tenant | Customer setup | Present | Keep as setup-only flow | Incremental enrichment only | Incremental polish only | P0 | Good foundation |
| Tenant | Customer detail | Present | Keep as detail surface | Readiness/payment enrichment | Detail enrichment | P0 | Good foundation |
| Pricing | Billable metrics | Missing | Fold into `Pricing` domain | Metering/catalog service, CRUD APIs | List/create/detail screens | P0 | Major missing core billing surface |
| Pricing | Plans | Missing | Fold into `Pricing` domain | Plan model/service/API | List/create/detail screens | P0 | Major missing core billing surface |
| Pricing | Add-ons | Missing | Fold into `Pricing` domain | Add-on model/service/API | List/create/detail screens | P1 | Important, but after metrics/plans |
| Pricing | Coupons / discounts | Missing | Fold into `Pricing` domain | Coupon model/service/API | List/create/detail screens | P1 | Important for go-to-market completeness |
| Pricing | Features | Missing | Keep secondary under pricing/subscriptions | Feature model/service/API | Detail and form surfaces | P2 | Avoid making this top-level too early |
| Pricing | Taxes | Missing | Expose as `Taxes` under billing config or pricing | Tax model/service/API | List/create/detail screens | P1 | Needed once invoices mature |
| Subscriptions | Subscription list and detail | Missing | `Subscriptions` domain | Subscription service and read APIs | List/detail surfaces | P0 | Core revenue-operating surface |
| Subscriptions | Create/update subscription | Missing | Guided subscription flow | Write APIs and validation | Setup/edit flows | P0 | Core revenue-operating surface |
| Subscriptions | Upgrade/downgrade | Missing | Secondary action on subscription detail | Change-plan workflow | Detail action flow | P1 | Important but after create/detail |
| Subscriptions | Alerts | Missing | Secondary surface under subscription detail | Alert model/service/API | Alert forms on detail | P2 | Keep out of top-level nav initially |
| Subscriptions | Entitlements | Missing | Secondary surface under subscription detail | Entitlement model/service/API | Detail action/forms | P2 | Keep out of top-level nav initially |
| Invoices | Invoices list/detail | Missing | `Invoices` domain | Invoice read/query APIs | List/detail surfaces | P0 | Core financial operations |
| Invoices | Manual invoice creation | Missing | Action from customer/invoice domain | Invoice create API | Create flow | P1 | Important, but after visibility surfaces |
| Invoices | Void/regenerate | Missing | Secondary actions on invoice detail | Void/regenerate APIs | Detail actions | P1 | Keep on detail, not top-level |
| Credits | Credit notes list/detail | Missing | Fold into `Invoices` or `Credits` | Credit-note APIs | List/detail surfaces | P1 | Important for finance ops completeness |
| Credits | Credit note creation | Missing | Secondary action from invoice detail | Create APIs | Create flow | P1 | Best placed under invoice context |
| Payments | Payment operations / recovery | Present | Keep as Alpha-owned ops surface | Continue enrichment | Continue polish | P0 | Already useful and differentiated |
| Payments | Payments list/detail | Missing | `Payments` domain | Payment read APIs | List/detail surfaces | P0 | Distinct from recovery-only surface |
| Payments | Manual payment creation | Missing | Secondary action in payment/invoice domains | Create APIs | Create flow | P1 | Needed for ops completeness |
| Payments | Overdue collection flows | Missing | Secondary action in customer/payment domains | Request/retry APIs | Action flows | P1 | Valuable after payment visibility exists |
| Wallets | Wallets and prepaid credits | Missing | `Credits` domain | Wallet model/service/API | List/detail/create/top-up | P1 | Valuable, but after invoices/payments |
| Recovery | Replay/recovery | Present | Keep as advanced tenant surface | Continue hardening | Continue polish | P0 | Advanced but valid Alpha-native surface |
| Explainability | Invoice explainability | Present | Keep as advanced tenant surface | Continue enrichment | Continue polish | P0 | Good differentiated capability |
| Analytics | Overview / attention widgets | Present | Keep Alpha summary style | Continue enrichment | Continue polish | P1 | Good foundation, not full analytics |
| Analytics | Revenue analytics | Missing | `Analytics` domain | Aggregation/reporting APIs | Dashboard/report screens | P2 | After revenue core exists |
| Analytics | Forecasts | Missing | `Analytics` domain | Forecast/reporting APIs | Forecast screens | P3 | Later-stage enhancement |
| Admin | Members | Missing | `Team & Security` domain | Membership APIs | List/detail/actions | P0 | Needed once Alpha owns auth |
| Admin | Invitations | Missing | `Team & Security` domain | Invite APIs, email flows | Invite flows | P0 | Needed once Alpha owns auth |
| Admin | Roles | Missing | `Team & Security` domain | Role model/service/API | List/detail/create/edit | P0 | Needed once Alpha owns auth |
| Admin | Authentication settings | Partial | `Team & Security` domain | SSO/provider config APIs | Auth settings UI | P0 | Tied to SSO rollout |
| Billing Config | Billing entities | Missing | `Billing Configuration` domain | Entity model/service/API | List/detail/create/edit | P1 | Needed once invoicing matures |
| Billing Config | Invoice custom sections | Missing | Secondary under billing configuration | APIs and templates | List/form surfaces | P2 | Avoid top-level complexity |
| Billing Config | Pricing units | Missing | Secondary under billing configuration | Model/service/API | List/form surfaces | P2 | Important later, not first |
| Billing Config | Email scenarios | Missing | Secondary under billing configuration | Email config APIs | Config surfaces | P2 | Important later |
| Billing Config | Dunning campaigns | Missing | `Billing Configuration` domain | Campaign model/service/API | List/detail/create/edit | P1 | Important for collections maturity |
| Integrations | Stripe billing connection | Partial | Keep Alpha-owned provider connection | Finish full lifecycle and status model | Continue billing-connection UI | P0 | Strategic priority |
| Integrations | Other payment providers | Missing | Extend `Billing Connections` | Provider adapters and lifecycle APIs | Reuse billing-connection UI | P1 | Add only after Stripe path is solid |
| Integrations | Accounting / CRM / tax | Missing | `Integrations` domain | Adapter/service/API work | List/detail/config screens | P2 | Product-value dependent |
| Developer | API key management for integrations | Missing | `Developer` or `API Access` domain | API key lifecycle APIs for machine auth | List/detail/create/revoke UI | P1 | Human browser auth should stay separate |
| Developer | Webhooks | Missing | `Developer` domain | Endpoint and delivery APIs | List/detail/log screens | P2 | Valuable but not ahead of billing core |
| Developer | Events / API logs / activity logs | Missing | `Developer` and `Audit` domains | Query/log APIs | Log viewer screens | P2 | Valuable for support and debugging |
| Portal | Customer portal | Missing | Separate customer-facing portal under Alpha branding | Portal auth/session/data APIs | Portal UX surfaces | P3 | Build after operator product is strong |

---

## Recommended Delivery Waves

These waves already reflect the market-signal lens:

- `Wave 1` = market table stakes Alpha must own
- `Wave 2` = operational and finance completeness
- `Wave 3` = enterprise maturity expansion
- `Wave 4` = later-stage completeness and ecosystem breadth

### Wave 1: Alpha must become a real billing control plane

- billable metrics
- plans
- subscriptions
- invoices list/detail
- payments list/detail
- members
- invites
- roles
- billing connections hardening

### Wave 2: Alpha must become operationally complete

- add-ons
- coupons
- taxes
- credit notes
- manual payment flows
- overdue collection flows
- dunning campaigns
- billing entities

### Wave 3: Alpha must become enterprise-ready

- auth settings and full SSO admin UI
- broader billing provider coverage
- accounting/CRM/tax integrations
- richer analytics

### Wave 4: Alpha must become ecosystem-complete

- developer tooling
- customer portal
- advanced forecasting

---

## Simplicity Guardrails

As this matrix is executed, these rules should remain in force:

1. Do not create top-level nav for every backend object
2. Prefer domain grouping over object sprawl
3. Keep advanced operations on detail pages
4. Avoid exposing backend engine terms in primary UI
5. Keep the product approachable for non-billing-expert users

---

## Update Rule

Whenever Alpha meaningfully adds or completes a major surface, update this matrix:

- move the `Alpha state`
- revise the `Priority` if needed
- adjust the delivery waves

This document is the working checklist for the Alpha import program.
