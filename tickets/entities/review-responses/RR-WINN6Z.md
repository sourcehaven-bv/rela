---
id: RR-WINN6Z
type: review-response
title: Duplicated paragraph in docs/data-entry.md; router test doesn't enforce the order it documents
finding: Two nits from the same review. (a) The 'Captured markdown is converted to HTML via goldmark' paragraph appeared twice in the Documents section — my edit re-added it below rather than moving it. (b) documentRoutes.test.ts documents that /document/:name/:entityId must be declared before /document/:name, but both assertions pass under either ordering, since vue-router ranks by specificity regardless of declaration order. The test documents an intent it does not enforce.
severity: nit
resolution: '(a) Removed the duplicate. (b) Left as-is with the comment intact: the ordering genuinely does not matter to vue-router, so there is nothing to enforce — the comment is now the accurate part and the test still usefully pins that both shapes resolve to distinct named routes with the right params. Noted here so the test is not mistaken for a regression guard on declaration order.'
status: addressed
---
