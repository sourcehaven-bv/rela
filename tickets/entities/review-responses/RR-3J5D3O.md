---
id: RR-3J5D3O
type: review-response
title: Basecamp PUT clears omitted fields
finding: PUT /todos clears description and assignees when omitted.
severity: critical
resolution: 'Plan R9: GET then resend unowned writable fields; stub clears omitted fields; survival test.'
status: addressed
---
