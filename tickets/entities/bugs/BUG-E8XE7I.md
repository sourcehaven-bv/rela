---
id: BUG-E8XE7I
type: bug
title: Denied document stays on screen after a failed SSE re-render
description: DocumentView and DocumentsPanel keep the previously rendered document visible when a re-render fails, regardless of cause. When the failure is a denial (401/403, or the read gate's uniform 404), content the principal may no longer read stays painted until they navigate away by hand.
priority: medium
effort: s
why1: The catch block in loadDocument() never clears docContent, so a render refused with 401/403/404 leaves the previous HTML on screen; only a toast reports the failure.
why2: BUG-DJZTRF removed the unconditional pre-fetch blank to stop the empty-state flash, and expressed the replacement rule as "keep whatever is already rendered" on ALL failures. That framing has no notion of a failure that withdraws the right to see what is kept.
why3: 'The blanking that BUG-DJZTRF removed had been incidentally doing double duty: it was written as a loading affordance, but it also meant a denied re-render could never leave stale content behind. Deleting it for the flash removed the security behaviour that nothing had ever named, so no test or comment recorded the loss.'
why4: The SPA has no shared vocabulary for "this rejection means access was withdrawn". ApiError carries a status, but every catch site re-derives intent from it ad hoc, so a rule about denials could not be stated once and reused - the two copy-pasted document loaders each had to get it right independently.
why5: 'Read-side ACL is designed and tested as a server-side property: the gate decides what a response contains, and correctness is asserted at the API boundary. Nothing extends that reasoning to content ALREADY delivered to a client that keeps it across refetches, so a view holding a stale authorized copy is invisible to every existing test and review path.'
prevention: 'Name the distinction in one place rather than at each catch site: shouldDropHeldContent() in api/errors.ts is the single classifier, and it is named for the DECISION (should a view drop content it holds) rather than the inferred cause, because 404 is a true result for it and must never drive a login redirect. Any surface that keeps fetched content on screen across a refetch owes a test for the denial case specifically, in its own file - the transient-failure case passing says nothing about it, which is why this regression shipped with a green suite, and a shared classifier still leaves one act-on-it line per component. Verify such a test by mutation, and check the assertion is not satisfied by something else in the same branch: the first draft asserted a cached badge was gone from a view that blanking had already unmounted, so it could not fail. The rule is now recorded in frontend/CLAUDE.md beside the keep-previous-content guidance it qualifies.'
status: done
---

## Symptom

A user is viewing a document in the data-entry SPA. Their access to the
underlying entity is revoked while the page is open (role change, ACL edit,
session expiry). The next SSE-triggered re-render is refused by the server, but
the document they were reading stays on screen. Only a toast reports the error.
The content remains visible until they navigate away by hand.

Reported as sourcehaven-bv/rela#1603 against CONTROL-8-03.

## Causal chain

1. Any entity write broadcasts `entity:changed` to every connected browser, so
`handleEntityChange` re-renders the open document (TKT-POT9GQ).
2. The server applies the read gate. A principal who may no longer read the
entry entity gets a uniform **404** (`TestACLDocuments_GatesHiddenEntity`); an
expired session gets 401, a refused capability 403.
3. `loadDocument`'s catch block surfaces the error and returns, deliberately
leaving `docContent` untouched — the BUG-DJZTRF rule.
4. The previously rendered HTML stays mounted.

## Scope

Client-side only, and narrow: the server already refuses correctly at every
point. This is about what the SPA does with a copy it was allowed to fetch
earlier and is no longer allowed to hold.

Both loaders carry the same copy-pasted shape and both are fixed, as in
BUG-DJZTRF — fixing one leaves the bug live on the entity page.

## Why 404 counts

rela's read gate answers a denied entity with a uniform not-found, because
whether an entity exists is itself a secret — `resolveAnchoredDocument` in
`internal/dataentry/export_document.go` mandates this in its own comment.

That sweeps in two non-denials, knowingly: an entity genuinely deleted, and a
document key dropped from `data-entry.yaml` (distinguishable by its
`document_not_found` code, and not confidential under the config-is-not-secret
rule). Both are content that no longer exists, so clearing the view is right for
them anyway. The cost of the ambiguity is an empty state instead of stale
content, not a wrong decision — which is why the code does not narrow on the
error code.

## Known gap this exposes: 401 has no recovery path

The SPA has no global 401 handling — `client.ts` has one interceptor and it only
normalizes the error; there is no login redirect anywhere. So an expired session
now yields a blank document, a toast, and a Refresh button that 401s again.
Blanking is still correct for confidentiality, but this turns a silent-staleness
bug into a visible dead end.

The gap predates this fix and is out of scope here; it is named rather than
discovered in production. A follow-up adds a global 401 handler, which must key
off `err.status === 401` directly and NOT off `shouldDropHeldContent` — a 404 is
a true result for that predicate, so wiring a login redirect to it would send
users to a login screen whenever they open a deleted entity.

## What is deliberately unchanged

A transient failure (network, timeout, 5xx, script error) still keeps content on
screen. That is BUG-DJZTRF's fix and it remains correct: the user keeps their
place, the toast explains the hiccup, and no authorization decision has been
made against them. Both test suites keep the test pinning it, directly beside
the new denial cases.
