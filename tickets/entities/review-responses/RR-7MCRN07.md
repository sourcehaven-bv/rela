---
id: RR-7MCRN07
type: review-response
title: Docs omit that transition guards check the caller
finding: A guarded transition still asks for the caller's permission, which the guide did not say.
severity: significant
resolution: The guide and the ScopedUpdates godoc say so and point to acl.yaml field rules. A test pins the 403.
status: addressed
---
