---
id: RR-QC44Q7
type: review-response
title: AC8 should also cover the read role's property and only this channel type's roles
finding: 'The handoff warning keys on "a named role''s property". `ct.Read` can also be `from: entity.<prop>` (`realtimecfg/validate.go:153-159`), and handing it off changes who can read the channel. Change: include `ct.Read` and limit the lookup to this channel type''s roles. Treat a composite template (text around `{{message.x}}`) like free text.'
severity: nit
resolution: The handoff warning covers the read role's property, only this channel type's roles, and treats composite templates as free text (AC 10).
status: addressed
---
