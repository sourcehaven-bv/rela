---
id: RR-W3AHRHN
type: review-response
title: Withheld scan returns empty slice not nil
finding: scanType returns an empty slice when withheld while other paths return nil.
severity: nit
reason: Both encode as an empty list on the wire; no caller distinguishes them.
status: wont-fix
---
