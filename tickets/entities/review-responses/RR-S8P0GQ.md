---
id: RR-S8P0GQ
type: review-response
title: Encode document kind in a type rather than an empty-string entryID sentinel
finding: 'resolvedDocument.entryID == "" means ''standalone'' in three places: handleV1ExportDocument''s resolver dispatch, RenderDocumentMarkdown''s engine dispatch, and exportBaseName''s filename stem. That is the same sentinel the router could accidentally manufacture from an empty path segment (RR-1JZTB0) - the empty-segment rejection removes the way to produce it, but the sentinel encoding remains. A kind field, or two small types, would make the empty-string case unrepresentable rather than merely unreached. The code reviewer noted that RenderDocumentMarkdown''s own godoc argues at length why "" and absent must not be conflated on the LUA side (entry_id nil vs empty string); the same argument applies to the Go side. Also flagged: frontend/src/api/documents.ts:28 interpolates path segments raw while the new transforms.ts documentExportUrl encodeURIComponents them - a pre-existing inconsistency, not a security issue since the server validates, but the two now visibly disagree about the same path shape.'
severity: minor
reason: 'Deferred: sound improvements, neither a defect in shipped behavior. The sentinel - RR-1JZTB0 closed the only way the router could manufacture an empty entryID, so the current code is correct; replacing the sentinel with a kind field would make the bad state unrepresentable rather than merely unreached, but it touches resolvedDocument plus three call sites, and doing it in the same diff as the security fix would blur which change closed the hole. The frontend encoding inconsistency is pre-existing in documents.ts and out of scope; changing it would alter a render URL builder this ticket does not otherwise touch.'
status: deferred
---

## Why deferred

Both items are sound, and neither is a defect in the shipped behavior.

**The sentinel.** RR-1JZTB0 closed the only way the router could manufacture an
empty `entryID`, so the current code is correct. Replacing the sentinel with a
kind field is a genuine improvement in making the bad state unrepresentable, but
it touches three call sites plus `resolvedDocument`, and doing it in the same
diff as the security fix would blur which change closed the hole. It is the
right follow-up, not the right amendment.

**The frontend encoding inconsistency** is pre-existing in `documents.ts` and
out of scope here; changing it would alter the render URL builder that this
ticket does not otherwise touch.

## If picked up

```go
type documentKind int

const (
    kindStandalone documentKind = iota
    kindAnchored
)

type resolvedDocument struct {
    cfg     documentRenderConfig
    kind    documentKind
    entryID string // empty iff kind == kindStandalone
}
```

Then `RenderDocumentMarkdown` dispatches on `kind`, and a future router bug
producing an empty id reaches the anchored branch and fails loudly instead of
silently rendering the standalone document.
