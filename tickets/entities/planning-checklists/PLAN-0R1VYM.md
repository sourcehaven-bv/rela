---
id: PLAN-0R1VYM
type: planning-checklist
title: 'Planning: Surface org_id and roles from verified identity assertions'
status: done
---

<!-- @managed: claude-workflow v1 -->

> **Revised after design review (10 findings, all addressed).** Two sections
> below were factually wrong in the first draft and have been corrected against
> the source: the `isUnstamped` behaviour (RR-HVN18E) and the
> `jwtauth → principal` dependency (RR-CHW2AA). Findings: RR-HVN18E, RR-3TKDR9,
> RR-QZVYVJ (critical); RR-CHW2AA, RR-7F05HH, RR-MZFKO7, RR-0VHKMW, RR-74XCVE,
> RR-TQBZ2U (significant); RR-9698LN (minor).

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

IN: typed assertion-claims projection in `internal/jwtauth` (with bounds);
`orgID`/`orgSlug`/`roles` on `principal.Principal` **as unexported fields behind
a `Verified` constructor**; asserted roles grantable via a new `acl.Policy` key
+ new `SourceKind` + one block in `computeGlobals`; threading through the
resolver chain and the audit sanitiser.

OUT: org **matching/enforcement** in `acl.yaml`; auto-provisioning an unmatched
principal (→ TKT-0C3II2); `--principal-header` gaining org/roles; the webhook
path; the `Authorization: Bearer` transport gap; `principal_type`.
**`isUnstamped` is explicitly out of scope — see the decision below.**

Full scope statement lives on TKT-RP3X3Q; not duplicated here to avoid drift.

**Acceptance Criteria:** AC1-AC10 on the ticket, each mapped to a test below.

## Research

- [x] ~~For larger features: run `/research`~~ (N/A: approach determined by
direct source study of both repos; no open option space left after the
org-scoping decision)
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A — see above.

**Existing Solutions:**

- **Libraries:** none needed. `golang-jwt` already parses the full claim set in
`VerifySubject` (verifier.go:90) and throws it away. This is a projection
change, not a parsing change. No new dependency.
- **Reusable code in-repo:**
  - `jwtauth.stringClaim` (verifier.go:167-172) — existing string-claim
projector; needs a `[]string` sibling.
  - `jwtauth.VerifyWebhook` (verifier.go:150-161) — the existing "verify, then
project into a typed struct" precedent.
  - **`metamodel.StringOrSlice` (metamodel/types.go:696-714)** — the in-tree
scalar-or-list YAML idiom, adopted for the policy key (see Approach).
  - `acl.EveryoneRole` injection (resolver.go:45-47) — role entering the
effective set without a graph walk.
  - `sanitizeUser` + `principalUserMaxLen` (dataentry/router.go:465) — the
existing input-bounding discipline, mirrored for roles.
  - `lua`'s `freezeTable` — the precedent for enforcing a contract structurally
rather than conventionally; the model for the `Verified` constructor.
- **Reference implementation:** `pratique/pkg/middleware/middleware.go:36-42` —
the vendor's own downstream contract exposes exactly `Subject`, `Email`,
`OrgID`, `OrgSlug`, `Roles`. This ticket mirrors that field set.
- **Prior art in the graph:** FEAT-OQBYHD, RES-SJNSUY, TKT-9089I6 (source of the
no-enumerable-principal problem via RR-CY6WYR).

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach — five layers, in dependency order:**

1. **`internal/jwtauth`** — add `AssertionClaims{Subject, Email, OrgID, OrgSlug,
Roles}` and `VerifyAssertion` beside `VerifySubject`. Same verifier, same pins;
only the projection widens. Add a `stringSliceClaim` helper. **Bounds live
here** (RR-0VHKMW): cap roles at 32 elements and each at 256 runes, drop excess,
`slog.Warn` once — so every future consumer inherits the bound instead of the
next entry point missing it. `jwtauth` **stays an arch-lint leaf** and does not
import `principal` (RR-CHW2AA).

2. **`internal/principal`** — `orgID`, `orgSlug`, `roles` as **unexported**
fields plus `Verified(sub, tool, orgID, orgSlug string, roles []string)` and
accessors (RR-TQBZ2U). Composite literals cannot set them, so all 21 non-test
construction sites are compiler-prevented from forging roles. Add `IsZero()`
(RR-3TKDR9). Custom `MarshalJSON`/`UnmarshalJSON` since `Principal` is embedded
in the published `audit.Record` wire format (audit.go:85-94). Hostile godoc on
org: *attribution only, nothing in `internal/acl` evaluates these, presence in
the audit log does NOT imply tenant isolation* (RR-9698LN).

3. **`internal/dataentry`** — `dataentry` declares **its own** small claims
struct; the `cmd/rela-server` adapter translates from `jwtauth.AssertionClaims`,
exactly as `webhookVerifierAdapter` does (main.go:244-252). `dataentry` does
**not** import `jwtauth` (RR-74XCVE). Populate via `principal.Verified` at
router.go:405. `resolvePrincipalEntity` (router.go:280) must carry the new
fields through its rebuild. **`internal/mcp/server.go:166` switches to
`IsZero()`.**

4. **`internal/acl`** — `AssertedRoleAssignments map[string]<StringOrSlice>` on
`Policy` (+ `knownPolicyKeys`, enforced by `policy_parity_test.go`), with a
local scalar-or-list `UnmarshalYAML` so both `admin: editor` and `admin:
[editor, auditor]` parse (RR-QZVYVJ). `acl` cannot import `metamodel`, so it
gets its own small equivalent of `StringOrSlice` — precedented, as `metamodel`
itself carries several such local YAML types. `SourceAsserted` kind with a
`Claim string` on `Source` (stays comparable), added to `sourceKindPriority`,
both `String()` methods, **and the `lessSource` tiebreak** (RR-74XCVE). One
block in `computeGlobals` adjacent to the everyone block. `Validate` rejects a
blank-after-trim key and rejects `EveryoneRole` as a mapping target (RR-QZVYVJ).
Claim matching is exact after `TrimSpace`, no case folding.

5. **`aclmap` + `aclaudit` + audit + docs** — a **distinct, separately-labelled**
"conditional grants (asserted claims)" section in `acl map`, explicitly NOT
reusing `EveryoneGrants` or its wire field (RR-MZFKO7). `Route` gains the claim
field + `lessRoute` tiebreak. `aclaudit` Tier-A check for "asserted claim maps
to a privileged role", cloned from `checkUngatedMembership`. `audit.sanitize`
gains a per-element `clean()` loop for roles, `clean()` for the org fields,
**and `RawUser`** which is currently unsanitised (RR-7F05HH). Docs per the
Documentation Planning section.

**Files to modify:**

- `internal/jwtauth/verifier.go`, `verifier_test.go`
- `internal/principal/principal.go`, `principal_test.go`
- `internal/dataentry/router.go`, `jwt_principal_test.go`, `principal_test.go`,
`principal_property_test.go`
- **`internal/mcp/server.go`** (added post-review, RR-3TKDR9)
- `cmd/rela-server/main.go`
- `internal/acl/policy.go`, `source.go`, `resolver.go`, and their tests
- `internal/aclmap/aclmap.go`, `whocan.go`, `enumerate.go`
- `internal/aclaudit/tier_a.go`
- `internal/audit/filesystem.go`
- `docs/acl-overview.md`, `docs/acl-security.md`, `docs/server-security.md`,
`docs/audit-log.md`

**Alternatives considered:**

- **Overload `Policy.Assignments` with claim values** — REJECTED. Its keys are
matched against `members` (entity IDs from a graph walk). A claim value
colliding with an entity ID grants across dimensions silently: a privilege
escalation vector, and unauditable.
- **`map[string]string` for the claim mapping** — REJECTED post-review
(RR-QZVYVJ). Forecloses `admin → [editor, auditor]`, and `acl.yaml` is a
published schema so widening later breaks operators. The scalar-or-list
unmarshaller gives the terse form for free.
- **Exported `AssertedRoles` field guarded by doc + test** — REJECTED
post-review (RR-TQBZ2U). For a field whose entire security property is "only
ever populated after signature verification", compiler enforcement beats
reviewer memory.
- **Mirroring `EveryoneGrants` for `acl map`** — REJECTED post-review
(RR-MZFKO7). `EveryoneGrants` is a statement of fact; an asserted grant applies
to an unknowable IdP-side subset. Reusing the global slot would tell an operator
everyone holds the role.
- **Full org enforcement** — REJECTED for this ticket by user decision;
`internal/acl` has no denial primitive and `RoleDef` verbs have no `when:`. Own
ticket, own design.
- **Org via the predicate env / via `inherit_roles_through`** — DEFERRED with
the above; noted as the natural first steps for the follow-up.
- **New source kind vs. reusing `SourceGlobal`** — new kind. Reusing
`SourceGlobal` would make audit and `acl map` unable to distinguish "granted by
policy assignment" from "granted by a claim in a token" — precisely the
provenance question asked post-incident.

**Dependencies:** no new modules. `jwtauth` gains **no** internal dependency and
remains an arch-lint leaf (RR-CHW2AA). `acl → principal` already exists.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

| Input | Source | Validation | On invalid |
|---|---|---|---|
| `roles` claim | verified JWT | signature verified BEFORE projection; must be `[]any` of strings; **capped at 32 × 256 runes** | non-string elements dropped, excess truncated with a warn; never errors the request |
| `org_id`/`org_slug` | verified JWT | string-typed, `clean()`-ed into audit | absent ⇒ empty string, not an error |
| claim→role mapping | `acl.yaml` (operator) | **allowlist**: mapped role must exist in `policy.Roles` or the grant is dropped (mirrors resolver.go:36-38); `Validate` rejects blank-after-trim keys and `EveryoneRole` targets | dropped at resolution / load-time error |
| role values reaching audit | derived | per-element `clean()` | sanitised |

The claim→role indirection is the primary control: a claim value never becomes a
role name directly, only via an operator-authored allowlist. An attacker able to
influence Pratique role names still cannot name a rela role the operator has not
mapped.

**Security-Sensitive Operations:**

- **Signature verification precedes projection.** `VerifyAssertion` parses claims
only from the verified token. A forged-key token yields no claims (reuses the
existing `_RejectsForgedKey` harness).
- **Roles are structurally unforgeable outside the JWT path** — the `Verified`
constructor is the only way to set them (RR-TQBZ2U). This replaces a
documentation-and-vigilance control with a compiler-enforced one.
- **Fail-closed preserved.** An invalid token still falls through the chain
(`_InvalidTokenFallsThrough`) and must not produce a Principal with roles.
- **`isUnstamped` is not touched** — see the decision below.
- **No leakage in errors.** 403 bodies already omit attributions
(`ForbiddenError.Error()`, acl.go:137-140); asserted provenance must not change
that.
- **Org is inert.** Nothing evaluates it; documented hostilely and pinned by
`TestOrgIsNotEvaluatedByACL` (RR-9698LN).

## Correction: the unmatched-principal path (RR-HVN18E)

**The first draft of this plan was wrong.** It claimed an unmatched verified
principal is "currently hard-denied by `isUnstamped`". Traced against source:

- `ResolvePrincipal` returns `("", nil)` on no-match (declarative.go:141-142).
- `resolvePrincipalEntity` hits `if id == "" || id == p.User { return ctx }`
(router.go:277-278) and returns ctx **unchanged**.
- `Principal.User` stays the verified `sub` — non-empty, not `"unknown"` — so
`isUnstamped` (request.go:190-196) returns **false**.

The request **already proceeds today** with zero roles. The comment at
router.go:265-268 documents this as intended ("a principal absent from the graph
is expected, e.g. a break-glass identity").

**Therefore AC10 requires no code change, and `isUnstamped` must not be
touched.** Relaxing it would weaken the fail-closed gate for every path, not
just the asserted one — the exact regression this correction exists to prevent.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**

| AC | Test |
|---|---|
| AC1 | `jwtauth`: ES256 token with all claims via the existing test signer ⇒ assert each `AssertionClaims` field. `dataentry`: stub ⇒ assert Principal accessors. |
| AC2 | Table: `roles: []`, absent, `null`, `org_id` absent ⇒ valid Principal, empty fields, no error, no fallthrough. |
| AC3 | Existing `TestHeaderPrincipalResolver_*` pass **unmodified**; plus empty org/roles assertions. |
| AC4 | Existing default-resolver tests unmodified; plus empty-field assertions. |
| AC5 | Policy `{admin: editor}` + roles `["admin"]` ⇒ `editor` attributed with `Kind: SourceAsserted, Claim: "admin"`. Also the list form `{admin: [editor, auditor]}` ⇒ both. |
| AC6 | Claim → undeclared role ⇒ zero attributions, no error, no panic. |
| AC7 | Record with org/roles/RawUser round-trips through `sanitize()`; control chars replaced, over-long elements truncated. |
| AC8 | Existing `nop_test.go` / `readonly_test.go` unmodified and passing. |
| AC9 | `aclmap` golden artifact: asserted grant appears in its own labelled section, **not** in the everyone/global slot. |
| AC10 | Unmatched verified principal proceeds with asserted roles only; `isUnstamped` unchanged. |

**Integration test:** unit tests are not sufficient — the real risk is a field
silently dropped between layers. Drive a real signed assertion through the
actual HTTP middleware stack (httptest JWKS, as `jwtauth`'s tests already do)
and assert (a) the audit record carries org and roles, (b) an ACL decision was
granted via `SourceAsserted`. This is what catches `resolvePrincipalEntity`
dropping fields; a resolver unit test would pass while the real path loses them.

**Trust-boundary table test (RR-TQBZ2U):** every non-JWT resolver — env
(router.go:348), header (router.go:321), default (router.go:295) — asserts empty
roles. First-class, not a prose bullet: `ChainResolvers` advances purely on
`p.User != ""` and ignores every other field.

**Edge Cases:**

- `roles: []` / absent / `null` ⇒ empty, no error, no panic.
- Non-string element (`["admin", 42]`) ⇒ drop the element, keep the rest.
- Over-cap: 33+ roles, or a 300-rune role ⇒ truncated with a single warn.
- Duplicate roles ⇒ deduped by the existing `attrKey` seen-map.
- Claim value colliding with an entity ID ⇒ must NOT grant via `Assignments`
(the separate-namespace property, tested explicitly).
- Claim value `" admin"` / `"Admin"` ⇒ no match (exact after trim); blank key
rejected at load.
- Two asserted attributions differing only by `Claim` ⇒ deterministic sort.
- Principal with roles but blank `Tool` ⇒ still rejected (guards the gate).
- Verified `sub` that is literally `"unknown"` ⇒ known pre-existing 500 via
`isBlankOrUnknown`; pinned by test, not fixed here.
- Empty/absent `asserted_role_assignments` ⇒ byte-identical to today.
- `TestOrgIsNotEvaluatedByACL` ⇒ two OrgIDs, identical decisions.

**Negative Tests:**

- Forged-key / `alg:none` / RS256 / expired ⇒ rejected, no claims, falls
through (existing harness).
- Header-mode request carrying a roles-ish header ⇒ no roles.
- Unknown top-level `acl.yaml` key ⇒ still warns (proves `knownPolicyKeys` was
updated, not bypassed).

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:** full list on TKT-RP3X3Q. The ones shaping implementation order:

1. **`SourceKind` additions are compiler-silent** — every switch has a default.
Mitigation: a reflective/exhaustive guard test, plus the `lessSource` tiebreak
(RR-74XCVE).
2. **`resolvePrincipalEntity` rebuilds the Principal field-by-field** — new
fields dropped by default. Mitigation: the end-to-end integration test.
3. **The `Verified` constructor touches 21 construction sites** and changes a
published JSON wire format. Mitigation: `IsZero()` for the `mcp` guard, custom
marshal/unmarshal, and existing audit round-trip tests.
4. **`acl map` mis-reporting** would be worse than under-reporting
(RR-MZFKO7). Mitigation: distinct section, golden-artifact test.

**Effort:** m, trending toward the upper end after review — the `Verified`
constructor and the custom JSON marshalling are more surface than the original
plan assumed.

## Documentation Planning

- [x] User-facing docs identified
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- [x] `docs/acl-overview.md` — the new key with both YAML forms, plus the
"Rules and fallbacks" list the existing keys carry (no-match, undeclared-role,
`EveryoneRole` rejection, absent-key no-op default).
- [x] `docs/acl-security.md` — trust boundary (roles only after signature
verification; a header must never be a role source); the 9-minute stale-role
window stated concretely (**a revoked admin keeps admin in rela for up to 9
minutes**, and rela has no revocation channel — an operator planning incident
response must know this); the org-is-not-isolation warning.
- [x] `docs/server-security.md` — the widened claim surface on the JWT path.
- [x] `docs/audit-log.md` — new fields, and the org-is-not-isolation warning.
- [x] `internal/principal` package doc — record that this is the ACL-gated
growth the doc anticipated, so the "don't grow speculatively" warning keeps its
force.
- [ ] ~~docs/metamodel.md, docs/cli-reference.md, docs/data-entry.md~~
(N/A: no metamodel, CLI or UI surface changes)

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** RR-HVN18E, RR-3TKDR9, RR-QZVYVJ (critical);
RR-CHW2AA, RR-7F05HH, RR-MZFKO7, RR-0VHKMW, RR-74XCVE, RR-TQBZ2U (significant);
RR-9698LN (minor). All 10 `addressed`; two corrected factual errors in the
original plan (RR-HVN18E, RR-CHW2AA).
