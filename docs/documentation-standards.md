# Documentation Standards

This document defines how Alpha docs should be written and maintained over time.

The goal is to keep the docs set useful for:

- current implementation work
- future contributors
- operational continuity
- architectural decision-making

---

## 1. Every Doc Must Have a Type

Every durable doc should fit one of these categories.

### Goal

Purpose:
- state long-term product or import intent

Example:
- [Alpha Import Goal](./alpha_import_goal.md)

### Model

Purpose:
- describe durable architectural boundaries and ownership

Examples:
- billing execution model
- workspace access model
- API credentials model
- notification architecture

### Spec

Purpose:
- define a concrete implementation slice or subsystem contract

Examples:
- workspace access spec
- API credentials spec
- slice specs

### Roadmap

Purpose:
- sequence work across slices and domains

Example:
- [Alpha Wave 1 Roadmap](./alpha_wave1_roadmap.md)

### Runbook

Purpose:
- explain how to operate or execute a workflow in a real environment

Examples:
- staging bootstrap
- infra rollout
- e2e payment runbook

### Checklist

Purpose:
- provide a finite, execution-oriented verification list

Example:
- staging go-live checklist

If a doc does not fit a category, it is probably not mature enough to be a durable reference yet.

---

## 2. Source-of-Truth Rule

For each important topic, there should be one primary source-of-truth doc type.

Examples:

- product intent:
  - `alpha_import_goal.md`
- Wave 1 sequencing:
  - `alpha_wave1_roadmap.md`
- workspace access implementation:
  - `alpha_workspace_access_spec.md`
- API credential implementation:
  - `alpha_api_credentials_spec.md`

Do not create overlapping docs that redefine the same topic in parallel.

If a newer doc supersedes an older one:

1. update links to the newer doc
2. mark the older doc as legacy in the docs index or inside the file
3. avoid silently leaving two competing references alive

---

## 3. Preferred Structure Inside Docs

For most model/spec docs, prefer this structure:

1. title
2. purpose/objective
3. scope
4. rules or constraints
5. target architecture or API/UI boundary
6. migration or rollout notes
7. testing or validation expectations
8. next steps or follow-on dependency

For runbooks, prefer:

1. purpose
2. prerequisites
3. exact steps
4. verification
5. rollback or failure handling

---

## 4. Keep Docs Focused

A doc should answer one main question well.

Good examples:
- how workspace access should work
- how notification ownership is split
- what Slice 4 invoices should include

Bad examples:
- mixing architecture, rollout steps, troubleshooting, and roadmap notes into one file

If a doc grows too wide:
- split by type or concern
- then link from the docs index

---

## 5. Prefer Additive Cleanup Over Renaming Churn

Do not rename files casually.

Renames should happen only when:
- the old name is materially misleading
- the document has become important enough that the name causes real confusion
- the benefit is worth broken references and git history churn

Default approach:
- keep existing files stable
- improve discoverability through the docs index
- improve clarity inside the document

---

## 6. Naming Rules Going Forward

Prefer names that reveal both topic and document type.

Good patterns:
- `*-model.md`
- `*-spec.md`
- `*-roadmap.md`
- `*-runbook.md`
- `*-checklist.md`

For Alpha slice continuity, existing `alpha_sliceN_*_spec.md` naming is acceptable and should remain consistent.

For new general docs, prefer kebab-case.

---

## 7. README Should Not Be the Docs Index

Root `README.md` should stay short.

It should:
- point to [Docs Index](./README.md)
- link a few entry documents
- avoid becoming a long unstructured list of every file

The docs directory should own detailed discoverability.

---

## 8. When to Update Docs

Update docs when:
- architecture changes
- ownership boundaries change
- a new product slice becomes the chosen implementation path
- a runbook changes operational reality

Do not wait for “perfect final state” if code or behavior has already moved.

Outdated docs are worse than missing docs if people rely on them.

---

## 9. Minimum Maintenance Rules for New Changes

When landing a meaningful new architecture or product slice:

1. update or add the relevant spec/model
2. update [Docs Index](./README.md) if the doc is durable
3. update the roadmap only if sequencing or priority changed
4. update runbooks only if operator behavior changed

Not every code change requires a docs change.
But every durable architectural or product decision should have one.

---

## 10. Legacy Docs Handling

A doc becomes legacy when:
- a newer source of truth replaces it
- the implementation no longer follows it
- it remains useful as historical context only

When that happens:
- mark it as legacy in the docs index
- optionally add a short note at the top of the file
- do not keep linking it as the active reference

---

## 11. Long-Term Goal

The long-term goal is not more docs.
The goal is a docs system where:

- the right starting point is obvious
- ownership is explicit
- implementation specs are easy to find
- runbooks are separated from architecture
- legacy planning notes do not compete with current truth
