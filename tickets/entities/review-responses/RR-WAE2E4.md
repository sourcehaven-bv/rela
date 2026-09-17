---
id: RR-WAE2E4
type: review-response
title: The view-traversal reachability leak is a LIVE read-side ACL bug in the view API and entity-detail sections, not just a command concern
finding: |-
    Follow-on from RR-3T18K9. Verified that both shared callers of executeView gate ONLY the entry, then serve the ungated traversal result to the user — so the reachability-via-hidden leak is already live on two shipped read surfaces, independent of commands:

    1. VIEW API (api_v1.go:2609-2645): gateReadOrNotFound gates the entry, then executeView runs the ungated multi-pass traversal, then buildSections(result) turns result.Collections into the wire ViewResponse sent to the browser. A hidden entity H on Entry->H->V means V (reachable only via H) appears in the _views response for a principal who could never GET it directly. api_v1.go:2611's own comment claims '_views is an entity-read chokepoint just like GET /{plural}/{id}' — but it is NOT: GET/{id} gates the one entity; _views gates only the entry and leaks the traversal closure.

    2. ENTITY-DETAIL SECTIONS (sections.go:341): same pattern via a synthetic ViewConfig; buildSections(result) feeds the entity-detail side panel. Same leak.

    CONSEQUENCE FOR TKT-2FDTJE's design decision (option a chosen): source-gating executeView's traversal is NOT scope creep — it fixes a live read-side ACL bug that DEC-ZBI39P's visibility-decorator model already says should be gated. Both callers get MORE correct (hidden intermediaries no longer leak reachable visible descendants), and neither should observably change for a principal who can read everything (NopACL / full grants) — that is the regression guard.

    The fix belongs at the traversal reader: traverseViewOnce (views.go) does st.GetEntity(targetID) on the raw store; route it (and the entry load in executeView) through the visibility seam (getVisible / PermitsRead), so a hidden target is neither placed in a collection nor recursed through. This is the internal/visibility 'read-out paths go through visibility wrappers' rule (CLAUDE.md, DEC-ZBI39P) applied to the one traversal reader that predates it.

    SCOPE NOTE: because this now fixes the view API + sections, the ticket's title/scope should acknowledge it is a read-side ACL fix for the view traversal generally, with commands as the motivating surface. Consider whether the NopACL byte-identical regression must be asserted for the view API + sections responses too (it should — that is the 'no behavior change for full-read principals' guard).
severity: significant
resolution: Split into BUG-9Z20WH, which fixes the executeView traversal at the source for all three consumers (_views API, entity-detail sections, view commands). The finding's core point — that this is a LIVE read-side ACL bug on shipped surfaces, not a command-only concern — is exactly why it warranted its own bug entity rather than being buried in a command-payload ticket. BUG-9Z20WH carries the NopACL byte-identical regression requirement for all three consumers and the 'chokepoint' comment correction at api_v1.go:2611.
status: addressed
---

## Why source-gating is the right call, restated

The leak was never command-specific. `executeView` is a **read-out traversal
reader** that predates `internal/visibility`, so it never got wrapped.
DEC-ZBI39P's rule ("read-out paths go through visibility wrappers, base readers
stay ungated") applies to it directly.

Gating at the source:
- fixes commands (this ticket's motivation),
- fixes the `_views` API leak (`api_v1.go:2611`'s "chokepoint" claim becomes true),
- fixes the entity-detail side-panel leak (`sections.go:341`),
- and is a no-op for any principal who can read the whole traversal (the NopACL / full-grant regression guard).

## Regression guard

Assert NopACL byte-identical output for **all three** consumers — command
payload, `_views` response, side-panel sections — so "no change for full-read
principals" is pinned, not assumed.
