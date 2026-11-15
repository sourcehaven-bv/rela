---
id: RR-CKW2XTP
type: review-response
title: Skipped and errored records reported as missing
finding: A record that failed before its key was derived or that the hook skipped made its entity show as missing.
severity: minor
reason: A hook skip is a deliberate removal from the source (the OSCAL hook skips withdrawn controls) so missing is right there. A record that fails before its key is known has no key to mark. The plan lists the error next to the missing entry, so the operator sees both.
status: deferred
---
