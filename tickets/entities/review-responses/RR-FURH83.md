---
id: RR-FURH83
type: review-response
title: canDrag duplicates helpers and checks inaccessible
finding: canDrag reimplemented the affordance helpers and used inaccessible (git-crypt) for redaction.
severity: minor
resolution: canDrag uses actionAllowed, isPropertyRedacted(_redacted) and isFieldWritable; the test case is now about _redacted.
status: addressed
---
