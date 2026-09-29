---
id: RR-QDO1NM
type: review-response
title: Function-name redaction on auth hid ACL decisions
finding: AuthorizeWrite arguments and deny errors were redacted.
severity: nit
resolution: 'secretFunc omits auth; parameter-name redaction keeps it. Test: TestSummarizeArgs/auth is not secret for functions.'
status: addressed
---
