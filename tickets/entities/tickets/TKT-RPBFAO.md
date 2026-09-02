---
id: TKT-RPBFAO
type: ticket
title: 'Restore the AC1.7 test: ACL deny returns a structured 403 body'
kind: test
priority: medium
effort: s
status: done
---

## Description

PR #1029 (god-object linter / affordanceService extraction) deleted
`internal/dataentry/acl_test.go` wholesale, taking with it
`TestHandler_ACLDeny_Returns403Structured` — the only automated test pinning
AC1.7:

> When `EntityManager` returns a `*acl.ForbiddenError`, the data-entry HTTP
> handler must respond with HTTP 403 and a structured JSON body carrying
> `rule_kind`, `rule_id` and `reason`.

GitHub issue #1044. Basis: CONTROL-8-29 (security testing in the development
lifecycle).

## Verified by mutation, not assumed

The gap is **partial**, which is worth stating precisely:

- Mutating `writeForbiddenIfACLDenied` to never fire (fall through to the
generic 500) **does** fail the suite — so the 403 *status* is covered
incidentally by other tests.
- Mutating the JSON body (`"error": "forbidden"` -> `"error": "MUTATED"`)
leaves the **entire `internal/dataentry` suite green**. The structured body —
the actual subject of AC1.7 — is pinned by nothing.

So the contract is half-covered, and the uncovered half is the one the AC is
about: an operator debugging a denial needs `rule_kind`/`rule_id`/`reason` to
know *which* rule fired. The AWS-IAM lesson the handler's own godoc cites
("opaque denials are unsupportable") is exactly what regressed silently.
