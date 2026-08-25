---
id: RR-O4VZW0
type: review-response
title: No test covers the sanitize/IsReserved control-char interaction
finding: 'principal_test.go''s IsReserved table covers whitespace (tab, leading/trailing spaces) but no C0/DEL control characters, and reserved_principal_test.go has no control-char case at all. A single table row (''\x01system:scheduler'') would have caught both RR-NQK412 and RR-OJRCNY. The planning checklist explicitly listed control chars as an edge case (''sanitizeUser turns them to spaces first ... still reserved by prefix'') -- that reasoning was written down but never pinned by a test, and it is only half true: sanitizeUser replaces control chars with SPACES, which TrimSpace then removes at the boundary, but IsReserved alone does not.'
severity: significant
resolution: 'Added the missing coverage at three levels. (1) TestIsReserved gained 7 rows: leading C0/DEL/NUL, mixed leading noise, interior control char (not reserved), control char after the prefix (reserved). (2) TestVerifiedPrincipal_RejectsReserved now asserts the REASON rather than just refusal, over a table of whitespace/C0/DEL/tab-prefixed subjects, plus the inverse case that a control-only subject reports rejectUnusable and not rejectReserved. (3) New end-to-end tests: TestRequireVerifiedJWT_ReservedSubjectLogsSecurityWarn (asserts the WARN fires AND the benign INFO does not, for plain/C0/DEL variants), TestRequireVerifiedJWT_UnusableSubjectStillLogsInfo (the discriminating direction), and TestStampAuditPrincipal_RejectsControlCharPrefixedReserved on the header path, which sets req.Header directly because http.Header.Set would reject a control char that a non-Go client can still put on the wire.'
status: addressed
---
