---
id: RR-2C6GDH
type: review-response
title: Echoed file value compared by base name only
finding: A save echoing a value with a different prefix but the same base name passed the rule and was stored as written.
severity: minor
resolution: An echo that passes the base-name check stores the stored value (pinStoredFileValues). Pinned by TestFileProperty_EchoKeepsStoredValue.
status: addressed
---
