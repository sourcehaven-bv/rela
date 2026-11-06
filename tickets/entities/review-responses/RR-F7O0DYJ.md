---
id: RR-F7O0DYJ
type: review-response
title: In-band token auth could clear the stamp
finding: '[security] handleAuth assigned cfg.StampAwareness unconditionally, so an OnTokenAuth hook returning a config without it would switch stamping off for that connection.'
severity: minor
resolution: A later auth frame may now replace the stamp or owner but never clear them; README and godoc state it. rela sets no OnTokenAuth.
status: addressed
---
