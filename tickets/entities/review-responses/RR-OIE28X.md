---
id: RR-OIE28X
type: review-response
title: 'Review cleanups: guard-test gap on the top-level menu, stale memo comment, overstated hasCreateForm doc, duplicate resolver call, unhelpful YAML error, nested-section row scan'
finding: 'Six smaller findings from code and security review. (1) assertViewSectionsLackKeys walked only sections[], so the new top-level ViewResponse.Create header menu was asserted absent nowhere — a regression defaulting the header placement on would have left the guard green. (2) creatableTargets'' doc comment described a `seen` memo that does not exist in that function (the only `seen` is in headerCreateMenu, a different concern), and the per-section re-authorization it claimed to avoid does happen. (3) hasCreateForm''s comment claimed to mirror createFormForType''s mode handling but checked only EntityType. (4) validateSectionCreate called SectionOriginRelation twice with the same inputs, discarding and re-fetching linkAs. (5) SectionCreate.UnmarshalYAML interpolated value.Value, which is empty for a sequence, so `create: [a, b]` produced `invalid create ""` with nothing locatable. (6) EntityDetail''s stillMissing check scanned only sec.entities, so a table (rows) or nested (tree) section always reported missing and fired the toast even when the row was on screen.'
severity: minor
resolution: '(1) Guard test now decodes and asserts the top-level create field too, with a comment on why a per-section scan cannot see it. (2) Comment replaced with what is actually true: not memoized, deliberately, because the work is metamodel/config/policy only (pinned by the query-budget test) and the affordance is opt-in so the section count is small. (3) Comment rewritten to say the check is deliberately looser and why the looseness is safe (createFormForType falls back to an edit-mode form, so the two agree on every input, and the runtime derivation is authoritative). (4) Bound linkAs from the first call. (5) Error now reports the node kind and line via a new yamlKindName helper. (6) Added sectionContainsEntity covering all three row shapes with a recursive tree walk.'
status: addressed
---

## Findings

Six smaller items from the two reviews, all addressed.

**1. Guard test could not see the new top-level field.**
`assertViewSectionsLackKeys` walked only `sections[]`, but `ViewResponse.Create`
(the header menu) is top-level. A regression that defaulted the header placement
on — or populated the menu without a `create:` block — would have left
`TestV1Views_NoAddOrLinkInfoOnSections` green while re-bleeding mutation onto
the read surface, which is the one thing it exists to prevent. The helper now
decodes and asserts the top-level field, with a comment saying why a per-section
scan is insufficient.

**2. Stale memo comment.** `creatableTargets`' doc described a `seen` memo that
is not in that function — the only `seen` is in `headerCreateMenu`, deduping
header entries by relation, a different concern — and the per-section
re-authorization it claimed to avoid does in fact happen. Replaced with what is
true: not memoized, deliberately, because the work is metamodel/config/policy
only (zero store reads, pinned by the query-budget test) and the affordance is
opt-in so the section count is small by construction.

**3. `hasCreateForm` doc overstated the check.** It claimed to mirror
`createFormForType`'s mode handling but tests only `EntityType`. Rewritten to
say the check is deliberately looser and why that is safe: `createFormForType`
prefers a non-edit form and falls back to an edit-mode one (RR-KGCF61), so "a
form exists" and "a form resolves" agree on every input — and the runtime
derivation is authoritative, so a future divergence fails toward a missing
button, not a broken one.

**4. Duplicate resolver call.** `validateSectionCreate` called
`SectionOriginRelation` twice with identical inputs, discarding `linkAs` and
then re-fetching it. Now bound from the first call.

**5. Unhelpful YAML error.** `UnmarshalYAML` interpolated `value.Value`, which
is empty for a sequence, so `create: [a, b]` produced `invalid create ""` — an
empty quoted string and nothing locatable, where every other validator in the
package names the view and section. Now reports the node kind and line via a
`yamlKindName` helper.

**6. Nested and table sections always reported the row missing.** `onCreated`
scanned only `sec.entities`, but a section's rows live in `entities`
(cards/list), `rows` (table), or `tree` (nested). So a table or nested section
always took the "created but not visible" branch and fired the toast even when
the row was plainly on screen — making the toast stop meaning what its comment
says. Added `sectionContainsEntity` covering all three shapes, with a recursive
tree walk.
