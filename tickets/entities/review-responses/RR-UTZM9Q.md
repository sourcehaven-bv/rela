---
id: RR-UTZM9Q
type: review-response
title: getCardFieldRawValue returned formatted text for relation fields
finding: A function named 'raw' returned the joined relation string ('A, B') for relation fields. It was harmless only because cardFieldWidget independently returns undefined for relations, so the value never reached a widget - two guards agreeing by luck. Adding a relation widget later would silently feed it pre-joined text.
severity: significant
resolution: Renamed to getCardFieldStoredValue and dropped the relation branch; it now handles property fields only and returns undefined otherwise. Callers use getCardFieldValue for relations explicitly.
status: addressed
---
