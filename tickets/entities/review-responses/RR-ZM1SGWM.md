---
id: RR-ZM1SGWM
type: review-response
title: False conflicts are not checked
finding: 'A merged body save while the user keeps typing leaves the base pre-merge, so the next save conflicts with the client''s own write. Add a property: the theirs side of each reported conflict is owned by the other client.'
severity: significant
resolution: Added the no-false-conflict property. The base of each refused request is read back from its precondition token; a conflicting field's server side must be the other tab's value or a lost write. It found two false conflicts, fixed in BUG-SJURXLT.
status: addressed
---
