---
id: RR-CDXR
type: review-response
title: Dead disjunct in isControlRune
finding: isControlRune(r >= 0 && r <= 0x1f) — for-range over a string never yields a negative rune (errors come back as utf8.RuneError = 0xFFFD). The r >= 0 disjunct is dead. Worse, it was copy-pasted into the audit-package twin.
severity: critical
resolution: 'Dropped r >= 0 from both copies. isControlRune is now `return r <= 0x1f || r == 0x7f`. Files: internal/dataentry/router.go:226-228, internal/audit/filesystem.go:268-270.'
status: addressed
---
