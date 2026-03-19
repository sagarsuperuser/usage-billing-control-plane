# Alpha Billing Execution Model

## Goal

Alpha should be the only product-facing control plane.

Users should think in:
- workspaces
- customers
- subscriptions
- payment setup

Users should not need to think in:
- Lago organization ids
- Lago provider codes

## Current reality

Today:
- Alpha owns the product workflow and secret storage
- Lago still owns the backing billing engine state
- a billing provider sync still needs a Lago organization context

That creates a leaky operator field:
- `lago_organization_id`

## Correct long-term model

Use two different concepts:

1. `Billing credential`
- platform-owned
- stores provider secret and high-level provider metadata
- does not ask normal operators for a raw Lago organization id

2. `Workspace billing binding`
- workspace-owned
- resolves the backing billing execution organization for that workspace
- connects the workspace to the billing credential

In that model:
- Alpha workspace is the product billing boundary
- Lago organization is only the backing execution boundary

## Transition rule

Until Alpha owns workspace billing bindings end to end:
- Alpha should resolve the backing Lago organization from platform configuration by default
- raw Lago organization entry should be treated as an internal override

That is safer than keeping the raw Lago organization field as a normal operator input.

## Implementation slices

### Slice 1
- support `BILLING_PROVIDER_DEFAULT_LAGO_ORGANIZATION_ID`
- billing connection sync uses the configured default when the connection does not override it
- UI treats raw Lago org input as an internal override

### Slice 2
- introduce explicit workspace billing binding
- move organization resolution closer to workspace provisioning
- stop treating billing connection as the concrete execution object

### Slice 3
- Alpha owns billing organization provisioning or resolution for each workspace
- remove raw Lago org id from the normal product flow entirely

## Current decision

This repo now implements Slice 1.

## Minimum seam set

These are the minimum seams Alpha must keep if we want to support stricter tenant isolation, compliance-driven separation, or dedicated billing execution contexts later without a rewrite.

### Must change now

1. Separate credential from execution boundary
- A billing connection must mean provider credential ownership, not the full execution boundary.
- Do not treat one billing connection as equivalent to one workspace billing context forever.

2. Keep workspace billing binding as a first-class future concept
- Workspace billing context must be able to exist independently from the billing credential.
- A future `workspace_billing_binding` or `tenant_billing_context` record should be able to point to:
  - workspace id
  - billing connection id
  - backend execution organization id
  - isolation mode
  - provisioning state

3. Avoid shared-org assumptions in service contracts
- Shared/default Lago organization can exist as a transition default.
- It must not be baked in as the permanent domain assumption.

4. Preserve derived mapping as derived
- `lago_organization_id` and `lago_provider_code` should remain implementation details.
- Alpha product logic should not require end users to understand or manage them during normal workflows.

5. Keep provisioning behind Alpha
- Any future creation or lookup of backend billing organizations should happen behind Alpha services/adapters.
- Users should not switch to Lago to complete the product workflow.

### Can wait

1. Dedicated workspace billing binding table
- We do not need to add it in the same slice as the default-org transition.
- But we should plan it as the next architectural step when isolation becomes active work.

2. Automatic backend organization provisioning
- Alpha does not need to create Lago organizations immediately.
- It does need a clean seam so this can be added later.

3. Shared vs dedicated isolation mode UI
- This can remain internal or config-driven until there is a real customer-facing need.

### Never hard-code again

1. One platform default org as the permanent model
- acceptable as a transition
- wrong as a long-term domain model

2. Workspace equals raw Lago org id in the product UI
- this is an internal mapping concern, not a stable product concept

3. Billing connection equals complete workspace billing identity
- that conflates credential and execution boundary

## Practical next architecture step

The next real model after Slice 1 should be:

1. `Billing connection`
- provider credential and provider-level metadata

2. `Workspace billing binding`
- workspace-owned binding to the billing execution backend
- owns the effective backend organization id
- owns isolation mode (`shared` or `dedicated`)
- owns provisioning state and audit trail

3. `Workspace provisioning`
- ensures the workspace has a valid billing binding
- resolves or creates the backing execution organization through Alpha

That is the smallest architecture that supports both:
- shared execution today
- dedicated isolated execution later

## Current decision filter

For any future billing work, apply this rule:

- if the change assumes all workspaces will always share one backend billing organization, do not bake that assumption into the domain model
- if the change keeps backend execution identity behind Alpha and leaves room for workspace-level binding later, it is acceptable
