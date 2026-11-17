---
id: RR-URAM6NA
type: review-response
title: 'Security: create.if_exists replace deletes without an ACL check'
finding: 'if_exists: replace calls DeleteEntity on the existing target with no ACL check; it was editable.'
severity: significant
resolution: 'create.if_exists is locked. Test: TestCheckEdit_Flows replace an existing target.'
status: addressed
---
