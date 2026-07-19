---
id: RR-IQZ62Y
type: review-response
title: applyCreateLock pins value only when Properties non-nil (asymmetry)
finding: applyCreateLock marks writable=false unconditionally but only pins result.Properties[prop]=entry when result.Properties != nil. If Properties were nil the field is locked read-only with no entry value on the wire — inconsistent. Properties is effectively always non-nil on a create candidate so this is defensive-only, but initialize the map if nil and pin unconditionally to remove the latent trap.
severity: nit
resolution: applyCreateLock now initializes result.Properties if nil and pins the entry value unconditionally (for visible machine fields), removing the writable=false-without-value asymmetry.
status: addressed
---
