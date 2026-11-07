---
id: RR-PAR9PDO
type: review-response
title: Redaction test checked no values
finding: TestElement_RedactedFieldDoesNotChangeViewHash set and then deleted the salary so the two values produced identical inputs.
severity: minor
resolution: Rewrote the test to state what it proves (the Redacted and Inaccessible lists do not count); hidden-value independence is pinned by TestEntityETag_HiddenEqualsAbsent.
status: addressed
---
