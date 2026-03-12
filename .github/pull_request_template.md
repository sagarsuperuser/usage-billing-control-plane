## Summary

- Describe what changed and why.

## Validation

- [ ] `go test ./...` passes locally
- [ ] `make test-integration` passes locally (real Temporal + Lago + Postgres/MinIO)

## Required CI Checks

- [ ] `migration-verify` is green
- [ ] `integration-temporal` is green

## Risk and Rollout

- [ ] Backward compatibility impact assessed
- [ ] Migration impact assessed (if DB changes are included)
- [ ] Rollback plan documented (if behavior is high impact)
