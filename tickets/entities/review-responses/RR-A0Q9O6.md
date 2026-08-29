---
id: RR-A0Q9O6
type: review-response
title: Dropped the already-discarded SchemaMetamodel parameter from writeEntityDetail (signature change, not a pure move)
finding: WriteSchemaEntityDetail took a third parameter declared as `_ SchemaMetamodel` — already discarded in the body on the base branch. The extraction dropped it rather than carrying a dead parameter onto the new type, so the call site became writeEntityDetail(resolved, def). This is a signature change rather than a strictly verbatim move, so it is recorded explicitly rather than left for a reviewer to spot in the diff.
severity: nit
resolution: 'Verified against the base commit: the parameter was literally `_ SchemaMetamodel` and unreferenced in the body, so removing it cannot change behavior or output bytes. Kept. The method is unexported on an unexported type with a single in-package caller, so there is no external API impact.'
status: addressed
---

Deviation self-reported by the implementing agent on TKT-NS3XPE (PR #1469) and
confirmed by the coordinator against base commit 11c02c1b.

Also confirmed on that PR: the pre-existing `tkt-ns3xpe-output-surface` branch
from the interrupted first attempt sat clean at develop with no commits, so it
was reused rather than recreated — no partial work was silently inherited.
