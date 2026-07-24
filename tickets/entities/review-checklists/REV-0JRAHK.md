---
id: REV-0JRAHK
type: review-checklist
title: 'Review: Surface org_id and roles from verified identity assertions'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated checks

- [x] `just test` — full suite passes
- [x] `just lint` — 0 issues
- [x] `just coverage-check` — PASS (76.3%, both thresholds)
- [x] `just arch-lint` — OK, no warnings (jwtauth stayed a leaf)
- [x] `go test -race` on all six touched packages — clean

## Code review

- [x] `cranky-code-reviewer` run against the branch diff
- [x] All critical/significant findings addressed

**Findings: 2 (1 significant, 1 minor). Both fixed.**

| ID | Severity | Finding | Status |
|---|---|---|---|
| RR-IK355A | significant | Padded `asserted_role_assignments` key loads clean but is permanently unmatchable | addressed |
| RR-S14ZKC | minor | `Principal` accessors handed out the roles backing array by value | addressed |

**RR-IK355A is a real bug the design review did not catch**, because it only
existed once the code was written: `Validate` rejected only all-blank keys,
while `computeGlobals` trimmed the incoming claim and then compared against the
untrimmed stored key. I reproduced it independently before accepting it — a `"
admin"` key loaded clean and matched nothing, in any form. Fixed by normalizing
keys in `Validate` (not `LoadPolicyBytes`, so no construction path bypasses it)
rather than patching the comparison. Both regression tests were fault-injected
to confirm they fail without the fix.

The reviewer also confirmed, by direct probing rather than inspection, several
properties I had asserted but not proven:

- `principal.Verified` has exactly two production call sites, both on the JWT
path; no reflection, no `unsafe`, no exported setter.
- `UnmarshalJSON` is unreachable from any request path — the only struct
embedding `Principal` with a JSON tag is `audit.Record`, and all 11 of its call
sites are write-side. The safety comment on the method is accurate.
- `org_id`/`org_slug` are genuinely inert: the only non-test uses of the
accessors are the re-stamp rebuild and `Roles()` in the resolver.
- The role bounds are non-bypassable — 100 junk elements followed by 40 strings
still yields exactly 32, because the cap is checked on output length.
- `truncateRunes` is multibyte-correct (300 `é` → 256 runes, no split codepoint).

**On `TestNonJWTResolvers_CannotSetAssertedClaims`:** I asked the reviewer
specifically whether my own comment — that this test may be structurally unable
to fail — was honest, and whether the test is worth keeping. Verdict: the
comment is accurate and the test should stay. It is a canary, not a proof; it
fails the moment someone exports the fields or hands a second resolver
`Verified`, which is the realistic regression. Its `got.User == ""` guard keeps
it from silently degrading into asserting nothing.

## Acceptance verification

All 10 ACs verified with named tests — see the table in IMPL-5NC7AC. Two guards
(the exhaustive `SourceKind` check and the re-stamp drop) were fault-injected
rather than assumed; both fail correctly with actionable diagnostics when the
fix is reverted.

Test-coverage gaps the reviewer identified, now closed:

- `TestAssertedRoles_PaddedClaimKeyIsNormalized` / `_PaddedKeysMergeRatherThanClobber`
- `TestPrincipal_AccessorsDoNotShareBackingArray`
- `TestNonJWTResolvers_ResolvePrincipalEntityIsTheOnlyOtherVerifiedCaller`

## Docs

- [x] `docs/acl-overview.md` — the new key, both YAML forms, rules and fallbacks
- [x] `docs/acl-security.md` — trust boundary, the 9-minute revocation lag
stated concretely, and an explicit "org is recorded, not enforced" section
- [x] `docs/server-security.md` — the widened claim surface
- [x] `docs/audit-log.md` — new fields plus the org-is-not-isolation warning
- [x] `internal/principal` package doc — records that this is the ACL-gated
growth the doc anticipated, and where the new limit now sits
- [x] `TestDocs_AclOverviewExampleWorks` parses the documented YAML verbatim,
so the example cannot rot silently
