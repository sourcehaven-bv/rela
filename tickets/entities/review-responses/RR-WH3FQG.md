---
id: RR-WH3FQG
type: review-response
title: Plan does not say where the Opener is built
finding: Tests swap app.acl after construction, so an Opener cached on App would miss the swap; responseGates cannot return an error.
severity: significant
resolution: attachACLRequest builds the Opener once per router behind the existing non-nil Declarative guard in responseGates; a construction error answers 500 acl_internal. The provision seam builds one per call from the live a.acl.
status: addressed
---
