# Alpha Import Goal

This document is the reference point for the Alpha import strategy.

The goal is not to clone Lago screen-for-screen. The goal is to make **Alpha** the single customer-facing control plane while **Lago** becomes an internal billing engine behind it.

---

## Product Goal

Alpha should be:

- the single entrypoint for operators and customers
- simpler and less intimidating than Lago
- role-aware and task-oriented
- the owner of authentication, permissions, navigation, and workflows
- the place where all major billing visibility and actions are available

Lago should be:

- an internal billing execution subsystem
- hidden behind Alpha service and UI boundaries
- used for capabilities, not exposed as the primary product experience

---

## Non-Negotiable Product Principles

### 1. Alpha owns the product experience

Users should experience Alpha, not Alpha plus Lago.

That means:

- one shell
- one auth model
- one permission model
- one navigation model
- one support story

### 2. Import capabilities, not UI complexity

We should not mirror Lago route-for-route.

We should import the capability, then redesign it into Alpha-native workflows.

### 3. Keep the UI simple and not intimidating

This is a hard constraint.

Implications:

- few top-level navigation items
- clear primary actions
- stable layouts
- progressive disclosure for advanced details
- friendly product language instead of engine/internal language

### 4. Use task-oriented information architecture

Alpha should prefer:

- `List`
- `Setup/Create`
- `Detail`

It should avoid overloaded screens that combine:

- create
- inventory
- diagnostics
- recovery

### 5. Keep role-specific complexity separated

Platform users should not see tenant/operator clutter.

Tenant users should not see platform plumbing.

---

## UX Constraints

Alpha should feel simpler than the system behind it.

That means:

- no raw Lago terminology in primary user flows unless unavoidable
- no dense operator dashboards as the default UI
- no exposing internal mapping fields as product concepts unless necessary
- advanced diagnostics should exist, but remain secondary

Examples of correct Alpha behavior:

- `Billing Connections` instead of raw provider-code plumbing
- `Needs attention` instead of backend error strings
- `Workspace`, `Customer`, `Plan`, `Invoice`, `Payment` as primary nouns

---

## Current Major Lago Frontend Surfaces

The current Lago frontend includes these major product surfaces.

Relevant route files:

- [front/src/core/router/index.tsx](https://github.com/getlago/lago/blob/main/front/src/core/router/index.tsx)
- [front/src/core/router/AuthRoutes.tsx](https://github.com/getlago/lago/blob/main/front/src/core/router/AuthRoutes.tsx)
- [front/src/core/router/CustomerRoutes.tsx](https://github.com/getlago/lago/blob/main/front/src/core/router/CustomerRoutes.tsx)
- [front/src/core/router/ObjectsRoutes.tsx](https://github.com/getlago/lago/blob/main/front/src/core/router/ObjectsRoutes.tsx)
- [front/src/core/router/SettingRoutes.tsx](https://github.com/getlago/lago/blob/main/front/src/core/router/SettingRoutes.tsx)
- [front/src/core/router/CustomerPortalRoutes.tsx](https://github.com/getlago/lago/blob/main/front/src/core/router/CustomerPortalRoutes.tsx)

### 1. Auth and access

- login
- signup
- forgot/reset password
- invitations
- Google auth callback
- Okta login and callback

### 2. Home, analytics, and forecasts

- home dashboard
- analytics
- forecasts
- usage analytics by billable metric

### 3. Customers and customer finance

- customers list and detail
- draft invoices
- customer invoice detail
- overdue payment request
- invoice void/regenerate
- credit note creation and detail

### 4. Catalog and pricing

- billable metrics
- plans
- coupons
- add-ons
- features
- taxes

### 5. Subscriptions and entitlements

- subscriptions list
- create/update subscriptions
- upgrade/downgrade
- alerts
- entitlements
- subscription details

### 6. Invoices, payments, and credits

- invoices list
- invoice creation
- payments list
- payment creation
- payment detail
- credit notes list/detail

### 7. Wallets and prepaid credits

- create wallet
- edit wallet
- top up wallet

### 8. Settings and billing configuration

- billing entities
- invoice sections
- pricing units
- taxes settings
- dunning campaigns
- billing-entity email scenarios
- billing-entity invoice settings

### 9. Integrations

- Stripe
- GoCardless
- Adyen
- Cashfree
- Moneyhash
- Flutterwave
- Anrok
- Avalara
- Hubspot
- Salesforce
- Netsuite
- Xero
- Lago Tax Management

### 10. Team and security

- members
- invitations
- roles
- authentication settings
- Okta configuration

### 11. Developer tooling

- API keys
- webhook endpoints
- webhook logs
- events
- API logs
- activity logs
- devtools surface

### 12. Customer portal

- portal init
- usage
- wallet
- customer information

---

## Alpha Product Direction

Alpha should become the single control plane for all major billing workflows.

That means Alpha should own:

- auth and SSO
- workspace and customer lifecycle
- pricing and catalog
- subscriptions
- invoices
- payments
- credits and wallets
- billing connections
- recovery and explainability
- analytics
- team/security
- developer tooling
- customer portal entry

This does **not** mean exposing Lago concepts directly. Alpha should translate backend capability into simpler product workflows.

---

## What Alpha Already Covers

Based on current Alpha work, these areas are already present or meaningfully started:

### 1. Auth and shell foundation

- dedicated browser login route
- human browser auth foundation
- SSO extensibility foundation
- role-based shell split

### 2. Platform surfaces

- overview
- workspaces list
- workspace setup
- workspace detail
- billing connections list/create/detail

### 3. Tenant surfaces

- customers list
- customer setup
- customer detail

### 4. Payment operations and debugging

- payment operations
- replay and recovery
- invoice explainability

### 5. Product and architecture direction

- setup/list/detail IA
- role-aware navigation
- provider-connect foundation
- Stripe-first billing-connection model

---

## What Is Still Missing In Alpha

If the goal is major Lago capability coverage through Alpha, the following remains pending.

### A. Core billing product breadth

These are the most important missing surfaces.

#### Catalog and pricing

- billable metrics
- plans
- add-ons
- coupons
- features
- taxes as first-class product UI

#### Subscriptions

- subscriptions list/detail
- create/update/upgrade-downgrade flows
- alerts
- entitlements

#### Invoices and credits

- invoices list/detail
- manual invoice creation
- void/regenerate flows
- credit notes list/create/detail

#### Payments and collections

- payments list/detail as a full product surface
- manual payment creation
- overdue collection flows
- receipt-related flows where needed

#### Wallets

- wallets
- wallet lifecycle
- wallet top-up

### B. Enterprise and admin surfaces

#### Team and security

- members
- invitations
- roles
- membership and invite management
- SSO/auth-provider admin UI

#### Billing configuration

- billing entities
- invoice custom sections
- pricing units
- billing email scenarios
- billing-entity invoice settings

#### Dunning

- dunning campaigns
- assignment of campaigns to billing entities

### C. Integration breadth

Alpha has a stronger product direction here than Lago, but functional breadth is still missing beyond the current Stripe-first path.

Missing or incomplete:

- GoCardless
- Adyen
- Cashfree
- Moneyhash
- Flutterwave
- Anrok
- Avalara
- Hubspot
- Salesforce
- Netsuite
- Xero
- tax-management integrations

### D. Customer-facing and developer surfaces

#### Customer portal

- portal bootstrap and access flow
- usage view
- wallet view
- editable customer information

#### Developer tooling

- webhook endpoints
- webhook delivery logs
- events browser
- API logs
- activity/audit logs
- mature API key management UI

### E. Analytics and forecasting

- analytics dashboards
- revenue views
- usage analytics breadth
- forecasts

---

## Recommended Alpha-Native Domain Model

Alpha should use a simplified, domain-oriented navigation model instead of copying Lago object-by-object.

### Platform domain surfaces

- Overview
- Billing Connections
- Workspaces
- Billing Configuration
- Team & Security
- Integrations

### Tenant domain surfaces

- Overview
- Customers
- Pricing
- Subscriptions
- Invoices
- Payments
- Credits
- Recovery
- Explainability
- Analytics

This is simpler and more usable than importing Lago navigation literally.

---

## Recommended Import Order

The import order should follow product value, not route count.

### 1. Revenue core

Highest priority:

- billable metrics
- plans
- add-ons
- coupons
- taxes
- subscriptions

Reason:

- Alpha cannot become the true product entrypoint for billing without owning pricing and subscription workflows.

### 2. Financial operations

- invoices
- credit notes
- payments
- overdue collection flows

Reason:

- these are core operator workflows and major parts of Lago's current value surface

### 3. Enterprise admin

- members
- invites
- roles
- authentication settings

Reason:

- once Alpha owns browser auth and SSO, it must also own admin lifecycle and access control UX

### 4. Billing configuration

- billing entities
- invoice sections
- pricing units
- dunning campaigns

Reason:

- these become necessary once Alpha owns invoicing as a product workflow

### 5. Integration expansion

- broaden billing connections beyond Stripe-first
- add tax/accounting/CRM integrations as product needs justify

### 6. Analytics

- dashboards
- revenue and usage views
- forecasts

### 7. Customer portal

- only after the main operator/control-plane product is strong

### 8. Developer tooling

- webhooks
- logs
- events
- API key admin

This is valuable, but should not outrank core billing product breadth.

---

## What We Should Not Do

### Do not clone Lago screen-for-screen

That will import too much complexity and too much backend-shaped language.

### Do not expose Lago internals as primary product concepts

Examples:

- raw provider codes
- raw billing-engine mappings
- engine-specific terminology in primary user flows

### Do not overload screens

Avoid mixing:

- setup
- list
- detail
- diagnostics
- recovery

on the same page.

### Do not make the UI feel intimidating

Avoid:

- dense admin dashboards as the default
- too many top-level nav items
- advanced operational metadata in the main workflow

---

## Decision Filter For Future Imports

For every Lago capability we consider importing into Alpha, we should evaluate it with the following questions:

1. What user job is this solving?
2. Does it deserve top-level navigation?
3. Can it be folded into a simpler Alpha-native workflow?
4. What is the minimum visible complexity needed for a strong first version?
5. What should remain advanced or secondary?
6. Does this expose engine internals that Alpha should hide?

If the answer trends toward complexity without strong user value, it should not be imported as-is.

---

## Working Conclusion

Alpha should become the only customer-facing control plane.

Lago should remain the billing engine behind Alpha.

The correct strategy is:

- import major billing capabilities
- redesign them into simpler Alpha-native workflows
- keep the UI approachable and role-aware
- avoid importing Lago complexity directly

This document is the working reference for that goal.

See also:

- [Alpha Import Matrix](./alpha_import_matrix.md)
  - includes the execution matrix plus market-signal classification for table stakes, enterprise maturity, and later-stage completeness
- [Alpha Wave 1 Roadmap](./alpha_wave1_roadmap.md)
  - breaks Wave 1 into concrete backend and UI delivery slices
