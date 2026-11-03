---
id: RR-5RXV27S
type: review-response
title: '[security] Unprefixed NOTIFY payload made every schema resync'
finding: '[security] notify.go handleNotify treated a rela_channels payload without the schema prefix as a gap for every handler. NOTIFY needs no privilege, so any database session could make every tenant''s processes resync once per 5 s.'
severity: minor
resolution: handleNotify now logs and drops such a payload. A real loss is still reported by gapAll when LISTEN runs again. TestNotify_DropsUnprefixedPayload sends a raw unprefixed pg_notify and checks that it is neither delivered nor counted as a gap.
status: addressed
---
