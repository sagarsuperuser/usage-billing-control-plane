# Alpha Market Gap Priorities

This document ranks the biggest remaining Alpha product gaps by market need.

Use it when deciding what to build next if the goal is:
- Alpha is the only normal product surface
- end users and operators never need Lago UI
- Alpha covers the most commercially important billing capabilities first

Read this together with:
- [Alpha Import Goal](../goals/alpha_import_goal.md)
- [Alpha Capability Ownership Map](../models/alpha_capability_ownership_map.md)
- [End-to-End Product Journeys](../runbooks/end-to-end-product-journeys.md)

## Ranking Rule

A gap ranks higher when it materially affects one or more of:
- revenue collection
- invoicing correctness and finance operations
- commercial flexibility in deals and packaging
- customer self-serve expectations
- market credibility against serious billing products

## Current Truth

Alpha is no longer missing the basic control-plane foundation.

These areas are already strong enough to stop being the main blockers:
- pricing setup
- subscription orchestration
- customer onboarding
- payment setup and collect-payment workflow
- payment retry/failure recovery surface
- replay and reconciliation
- invite and access lifecycle
- browser/operator baseline journeys

So the next priorities should not be chosen as if Alpha were still missing the basics.

## Must-Build Now

### 1. Dunning

This is the biggest market-facing gap.

Why it matters:
- failed payments and overdue invoices directly affect revenue collection
- buyers expect more than manual retry buttons and operator guidance
- this is a standard collections capability in mature billing products

What Alpha has today:
- lifecycle guidance
- manual retry
- collect-payment workflow
- overdue and failure visibility

What Alpha still lacks:
- dunning policy model
- scheduled retries
- reminder cadence
- escalation rules
- per-invoice dunning state
- operator controls for pause/resume/override
- notification history tied to collections state

Why this is first:
- it closes the clearest gap between Alpha's current recovery UX and market expectations for revenue recovery

### 2. Taxes, Billing Entities, and Invoice Configuration

This is the biggest finance-operations and enterprise-readiness gap.

Why it matters:
- real customers quickly need tax settings, invoice settings, and sometimes multiple billing entities
- without this, product credibility drops for companies with more than the simplest billing footprint

What Alpha still lacks:
- first-class tax configuration
- billing entities product workflow
- invoice custom sections and related invoice configuration depth
- stronger invoice policy controls at the product layer

Why this is second:
- dunning helps collect money already owed
- this layer is what makes invoicing and finance setup believable for broader real-world customers

### 3. Discounts, Add-ons, and Commercial Packaging Depth

This is the biggest commercial flexibility gap.

Why it matters:
- real deals often need more than a clean plan + meter model
- startups and mid-market teams regularly need:
  - discounts
  - add-ons
  - richer packaging and exceptions

What Alpha still lacks:
- add-ons as a first-class Alpha product surface
- coupons and discounts
- richer commercial packaging controls beyond the current strong core pricing path

Why this is third:
- many real go-to-market motions depend on this before they depend on deeper later-stage analytics

## Build Next

### 4. Customer Portal

Why it matters:
- if Alpha is the only customer-facing product, customers eventually need their own billing surface under Alpha branding
- this is a market expectation once operator-side workflows are strong enough

Expected scope:
- invoice visibility
- payment method visibility / management
- subscription visibility
- possibly usage visibility
- basic self-serve billing profile updates

Why it is not in must-build-now:
- operator control plane maturity still matters first
- but it becomes important soon after the commercial core is strong

### 5. Usage-to-Issued-Invoice Maturity

Why it matters:
- Alpha already proves current billable state and explainability on known invoices
- the next important product proof is:
  - real usage
  - real issued/finalized invoice
  - Alpha invoice visibility on that exact invoice
  - Alpha explainability on that exact invoice

Why it matters commercially:
- customers and operators care about what was actually issued, not only current usage or recovery state
- this makes the invoicing story feel more complete and real

### 6. Billing Connection Lifecycle Completion

Why it matters:
- Alpha should fully own create, verify, rotate, repair, and workspace binding workflows
- backend/provider repair knowledge should not leak into normal product operations

Why it is not above dunning or taxes:
- it is strategically important, but slightly more platform-operational than market-visible for many buyers
- it still matters a lot for product credibility and supportability

## Later-Stage Completeness

### 7. Wallets and Prepaid Credits

Important for:
- credit-based billing models
- prepaid usage businesses
- certain usage-heavy SaaS models

Valuable, but not ahead of the larger revenue-collection and invoicing gaps.

### 8. Broader Provider and Integration Coverage

Important for:
- larger deals
- multi-provider requirements
- accounting and CRM workflows

But the correct sequence is:
- first make the Stripe-first path product-complete
- then broaden provider and integration coverage

### 9. Richer Analytics and Forecasting

Important, but later.

Why:
- analytics help optimize and report
- they do not matter more than collecting revenue correctly, configuring invoices correctly, and handling commercial structure cleanly

### 10. Developer Tooling Breadth

Includes:
- webhook endpoints
- webhook logs
- API logs
- activity logs
- broader devtools surfaces

Useful for mature platform and support operations, but not the biggest current market blocker.

## Recommended Build Order

If the goal is strongest market progress from the current foundation, use this order:

1. Alpha-owned dunning
2. taxes + billing entities + invoice configuration
3. discounts + add-ons + commercial packaging depth
4. usage-to-issued-invoice journey and product maturity
5. billing connection lifecycle completion
6. customer portal
7. wallets / prepaid credits
8. broader provider and integration coverage
9. richer analytics
10. broader developer tooling

## Why This Order Is Correct

This order follows the actual commercial priority chain:

1. collect money reliably
2. issue invoices correctly and support finance operations
3. support real pricing and packaging deals
4. prove invoicing and explainability on real issued billing state
5. remove backend leakage from core operations
6. expand self-serve and ecosystem breadth

That is a stronger market path than focusing early on wider analytics or broad peripheral tooling.

## Practical Decision Rule

When choosing between two roadmap items, prefer the one that better improves:
- collections maturity
- invoicing correctness
- commercial packaging flexibility
- Alpha-only customer/operator experience

De-prioritize items that mostly improve breadth without solving those core commercial needs first.
