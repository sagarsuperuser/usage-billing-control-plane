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
