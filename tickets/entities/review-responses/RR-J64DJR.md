---
id: RR-J64DJR
type: review-response
title: 'Design: elevated readers return deleted rows without history:read'
finding: 'UnrestrictedReader (NopACL and allow_acl_bypass: read) lists deleted entities without the history:read check.'
severity: nit
reason: 'Intended: an unrestricted reader holds the read-everything capability by construction (DEC-ZBI39P), and its other history reads are ungated too.'
status: wont-fix
---
