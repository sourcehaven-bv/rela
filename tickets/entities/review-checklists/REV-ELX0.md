---
id: REV-ELX0
type: review-checklist
title: 'Review: loader allowlist parity tests (dataentryconfig + acl)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] Both new parity tests pass (`TestValidTopLevelKeysMatchConfigStruct`, `TestKnownPolicyKeysMatchStruct`)
- [x] Lint clean on both packages (`golangci-lint run ./internal/dataentryconfig/... ./internal/acl/...` → 0 issues)
- [x] ~~Coverage~~ (N/A: test-only change, no production code touched — floors cannot drop)

## Manual Review

- [x] Each test verified to bite: removing an allowlist entry (`palette` / `inherit_roles_through`) fails the corresponding test; restoring passes
- [x] Confirmed both allowlists are currently in sync with their structs — purely additive coverage, no latent bug found
- [x] `acl` test placed in an internal-package file (`policy_parity_test.go`) because `knownPolicyKeys` is unexported
- [x] Mirrors the established `metamodel.TestValidTopLevelKeysMatchStruct` pattern (RR-GHDQXC follow-up from BUG-5XIN07)
- [x] PR: https://github.com/sourcehaven-bv/rela/pull/1103
