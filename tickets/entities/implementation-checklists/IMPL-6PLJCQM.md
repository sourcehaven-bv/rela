---
id: IMPL-6PLJCQM
type: implementation-checklist
title: 'Implementation: Remove peer keys; machine clients authenticate at the identity edge'
started: "2026-11-08"
completed: "2026-11-08"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] Integration tests written (test full flow, not just units)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (errors surfaced, not swallowed)

New tests: `internal/secrets/global_test.go` (`TestScriptReadable`,
`TestGlobal_IgnoresOverrides`), `TestReconcileDiff_TokenSecret` (real
`.rela/secrets.yaml`, httptest remote checks the bearer value; refusals for
`token_key`, `api_key`, the bare prefix, a missing, override-only and empty
secret, no secrets source, two credential flags),
`TestFilterSecrets_DropsUnreadableSecrets` (lua, both grant shapes),
`TestScriptConfig_RefusesUnreadableSecrets` (mail.yaml), a metamodel table row.
The tests of the removed code are deleted with it; the reverted tests
(`reserved_principal_test.go`, `jwtgate_test.go`, `principal_test.go`,
`acl_middleware_test.go`, `asserted_e2e_test.go`, `principallookup_test.go`) are
back to their pre-#2156 form.

Found while restoring the generic rule: `lua.Capabilities.filterSecrets` under
`AllSecrets` dropped only `token_key`, so in #2156 a `peer_credential.` secret
reached an all-secrets script. It now filters through `secrets.ScriptReadable`.

## Test Quality

- [x] Using fixture builders or factories for test data
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

- AC1: `rela peer key create x` answers `unexpected argument peer`;
`grep -riE "peerkey|relapeer|peer_key|TypePeer|PeerPrefix"` over Go and YAML
finds nothing; `go build ./...`, `go vet -tags postgres ./cmd/rela-server/` and
`just arch-lint` pass.
- AC2: `reserved_principal_test.go` (reverted) passes; `peer:` is not in
`principal.IsReserved`.
- AC3: built CLI against a header-mode `rela-server -principal-header
X-Remote-User` on a copy of `examples/relation-status-demo`, with
`remote_credential.up` in the client's `.rela/secrets.yaml`: `rela reconcile
diff --remote http://127.0.0.1:18093 --auth-header X-Remote-User --token-secret
remote_credential.up` answered "No differences", 1 request. `--token-secret
api_key` is refused with "must start with remote_credential."; without a secrets
file the error names the missing source; `--token-secret` with `--token-env` is
refused by kong.
- AC4: unit tests above, for schema, Lua grant and mail.yaml.
- AC5: docs reviewed after `just docs`; no "Peer keys" section remains.
- Package tests: `go test` on dataentry, principal, acl, cli, secrets, lua,
metamodel, mail, reconclient and cmd/rela-server pass.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
