---
id: RR-Y85M
type: review-response
title: Live re-derivation is a UI hint only; plan must not let it gate commit client-side
finding: 'External research (Salesforce/Sanity/JSONForms) is unanimous: value-dependent client gating is a UI hint, never the authorization boundary. The plan''s AC says ''block-submit on hard denials'' — this must be a UX nicety layered on TOP of the server re-authorizing at commit, NOT a replacement. The commit (POST create) MUST run the full BUG-Q60V gate regardless of what the live derivation said; a client that ignores the hint and POSTs anyway must still 403. Plan should state explicitly: dry-run verdicts are advisory; the create write is the only authorization point. Add a test: POST create with a denied field succeeds-as-403 even if no prior dry-run occurred (already covered by BUG-Q60V tests — reference them so this invariant is pinned).'
severity: significant
resolution: 'Plan updated: dry-run verdicts are advisory UI hints only. The commit (POST create) runs the full BUG-Q60V gate regardless of any prior dry-run; a client that POSTs a denied field still 403s. Pinned by existing BUG-Q60V tests (TestHandleV1CreateEntity_FieldAffordances) plus a new assertion that a commit with no prior dry-run still gates. External-systems research reinforced this as the universal invariant (UI hint, re-authorize on commit).'
status: addressed
---
