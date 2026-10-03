---
id: RR-P12I7P
type: review-response
title: Other zero-face lookups remain outside the delete cascade
finding: cascadehost.go (IfExistsReplace delete), core.go findExistingRelationTarget, rename.go and apply.go requireEndpoint still read the zero face and miss faced entities.
severity: significant
reason: Out of scope for BUG-58BL9I, which covers the delete-cascade relation check. RenameEntity is fixed in parallel by BUG-Y1RGTU. The rest are pinned by the archguard zero-face allowlist, which may only shrink, and are part of the DEC-NPZICR staged sweep (RES-Y6JA37).
status: deferred
---
