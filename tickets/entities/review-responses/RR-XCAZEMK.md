---
id: RR-XCAZEMK
type: review-response
title: Entity rows and relations use different gate handles
finding: conflictsReadable takes vr but gates entity rows through ctxRowGate directly.
severity: minor
resolution: Not fixed in this change; see reason.
reason: vr's resolver is built over ctxRowGate, so both halves ask the same per-request gate; visibleReader exposes no face-verdict method for an unloaded row. The doc comment now states this.
status: wont-fix
---
