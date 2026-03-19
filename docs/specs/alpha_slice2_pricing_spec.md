# Alpha Slice 2 Spec: Pricing Foundation

This document defines the second Wave 1 implementation slice for Alpha: the first real tenant-side pricing domain.

Read together with:

- [Alpha Import Goal](../goals/alpha_import_goal.md)
- [Alpha Import Matrix](../goals/alpha_import_matrix.md)
- [Alpha Wave 1 Roadmap](../roadmaps/alpha_wave1_roadmap.md)

---

## Objective

Alpha must let tenant operators define the core pricing backbone without leaving Alpha.

For Wave 1, that means Alpha should own:

- billable metrics
- plans

This is the minimum strong pricing foundation. It is intentionally narrower than full Lago pricing breadth.

---

## Product Scope

### In scope

- Pricing top-level domain
- billable metrics list/create/detail
- plans list/create/detail
- simple relationships between plans and billable metrics as needed for the first strong version

### Out of scope

- add-ons
- coupons
- advanced feature management
- taxes as a full product surface
- highly complex fee-shape UX
- every Lago pricing configuration surface on day one

---

## User Stories

1. A tenant operator can browse all pricing objects from a single `Pricing` domain.
2. A tenant operator can define a billable metric in Alpha.
3. A tenant operator can define a plan in Alpha.
4. A tenant operator can inspect plan and metric details in Alpha.
5. A tenant operator can create subscriptions later using pricing defined in Alpha.

---

## Product Rules

1. `Pricing` should be a domain, not a dump of disconnected billing objects.
2. Keep the first version simple enough for non-billing-expert users.
3. Use progressive disclosure for advanced pricing options.
4. Prefer guided create flows and detail pages over dense form consoles.
5. Do not expose Lago object complexity unless there is real product value.

---

## Target Product Surface

### Routes

- `/pricing`
- `/pricing/metrics`
- `/pricing/metrics/new`
- `/pricing/metrics/[id]`
- `/pricing/plans`
- `/pricing/plans/new`
- `/pricing/plans/[id]`

### Navigation placement

- one top-level tenant nav item: `Pricing`

Within Pricing:

- Metrics
- Plans

Do not add top-level nav for:

- add-ons
- coupons
- features
- taxes

at this stage.

---

## Backend Scope

### Domain boundary

Alpha should own a pricing service boundary that exposes Alpha-native pricing concepts even if execution uses Lago-backed capabilities underneath.

### Required backend work

1. Billable metrics model and APIs
- create
- list
- get detail
- update if needed for first version

2. Plans model and APIs
- create
- list
- get detail
- update if needed for first version

3. Relationship support
- enough linkage between plans and billable metrics for the subscription flow that follows

4. Validation rules
- stable names/codes
- required fields
- sane first-version constraints

5. Permission model
- tenant-scoped only
- view vs write permissions should be explicit

### Suggested APIs

Use Alpha-owned tenant APIs for:

- billable metrics CRUD
- plans CRUD

The exact route shape can vary, but the product model should remain:

- tenant-scoped
- Alpha-native
- suitable for list/detail/create flows

### Response expectations

Responses should emphasize:

- metric or plan name
- code/identifier
- status where applicable
- essential commercial meaning
- timestamps

Responses should avoid:

- dumping raw engine-specific configuration that users do not need in the first version

---

## UI Scope

### Pricing landing

Must provide:

- clear entry into Metrics and Plans
- simple explanation of what each is for
- strong CTA to create the first pricing object

### Metrics list

Must show:

- name
- code
- essential type/shape summary
- usage count if useful and cheap

Must support:

- create
- inspect detail

### Metric create

Must optimize for:

- minimal required choices
- clear examples
- low intimidation

### Plans list

Must show:

- plan name
- code
- essential pricing summary
- status if applicable

Must support:

- create
- inspect detail

### Plan create

Must optimize for:

- a simple guided path
- reasonable defaults
- minimal advanced configuration in the initial version

### Detail pages

Must provide:

- summary first
- related objects
- next actions

Advanced sections may include:

- deeper pricing configuration
- engine-derived details

but should not dominate the page.

---

## UX Notes

This slice is high risk for complexity creep.

Guardrails:

1. Do not import every Lago pricing option immediately.
2. Keep forms progressive and teach the user what matters.
3. Separate list/create/detail cleanly.
4. Avoid turning the first version into a billing-expert-only configuration tool.

Preferred user framing:

- Metric = what gets measured
- Plan = how customers are charged

That is simpler than exposing internal billing object jargon first.

---

## Testing Requirements

### Backend

- service tests for metrics and plans behavior
- API tests for:
  - tenant scoping
  - permission checks
  - validation rules
  - list/detail/create flows

### UI

- tenant session route coverage
- list/detail/create coverage for metrics
- list/detail/create coverage for plans
- empty-state coverage
- wrong-role coverage

### Staging

- create a metric
- create a plan
- verify both appear in lists and details
- confirm these objects are available for the next subscription slice

---

## Exit Criteria

This slice is complete when:

1. A tenant operator can define and inspect billable metrics in Alpha.
2. A tenant operator can define and inspect plans in Alpha.
3. The `Pricing` domain feels simple and approachable.
4. Alpha no longer depends on Lago UI for core pricing foundation workflows.
