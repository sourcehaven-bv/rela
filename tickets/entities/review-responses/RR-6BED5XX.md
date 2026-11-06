---
id: RR-6BED5XX
type: review-response
title: Renumber judged siblings before its Tx so a new edge could be rewritten unjudged
finding: renumberAllowed listed and judged the siblings before the side's Store.Tx; maybeRenumberSide listed them again inside the Tx and rewrote all of them. An incoming edge created in between was renumbered without its source being judged.
severity: significant
resolution: renumberAllowed returns the set of edges it judged; maybeRenumberSide rewrites nothing when the Tx lists an edge outside that set. Pinned by TestMaybeRenumberSide_SkipsUnjudgedSibling. A compiled-policy test on the field-gated handle (TestFieldGate_IncomingMoveJudgesEverySource) was added too.
status: addressed
---
