---
id: RR-E6T54VN
type: review-response
title: missingClaim derives requirements by exclusion
finding: needUser and needOrg were computed from what the family is not so a new family silently inherits requirements.
severity: minor
resolution: Explicit switch per family; an unhandled family requires every claim and so fails closed.
status: addressed
---
