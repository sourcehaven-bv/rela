---
id: RR-BU24VR1
type: review-response
title: Gated path appends to a possibly shared Props slice
finding: base.Props is appended on a shallow copy of the ACL query.
severity: minor
resolution: No change.
reason: gated already clones base.Props (slices.Clone) before appending; the finding does not apply.
status: wont-fix
---
