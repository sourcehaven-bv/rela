---
id: RR-D2L2XUQ
type: review-response
title: "Lookup tables match inherited keys"
finding: "Switches turned into plain-object lookups return Object.prototype members such as `constructor` for outside strings instead of the default."
severity: minor
resolution: "Added utils/ownLookup (own-property check) with a test and used it at all six sites."
status: addressed
---

Switches turned into plain-object lookups return Object.prototype members such as `constructor` for outside strings instead of the default.
