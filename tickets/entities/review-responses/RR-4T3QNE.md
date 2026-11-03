---
id: RR-4T3QNE
type: review-response
title: The guide says unread must be true or false, but more values load
finding: docs-project/entities/guides/GUIDE-realtime.md:274 says "`unread` must be `true` or `false`". In fact yes/no/on/off/y/n load, and so do quoted "no" and "yes" (internal/realtimecfg/unread_test.go, row "quoted no"). This is deviation 1, against AC 1, which said a quoted "no" is refused. Either describe this accurately in one clause (for example, "a YAML boolean; quoted words such as \"no\" are read as booleans too") or record in the ticket that AC 1 changed. As written, the docs and the AC both disagree with the code.
severity: minor
resolution: 'ADDRESSED chose the strict option, matching confirm: unread is a rawUnread node; unreadOf takes only a !!bool, refuses quoted and !!str values with "must be true or false" and unquoted yes/no/on/off/y/n via yamlscalar.PlainYAML11Bool. AC 1 holds as written (quoted "no" refused). Guide states the rule; TestParse_Unread rows cover tagged bool, null, unquoted no/Off, quoted no/false, !!str no.'
status: addressed
---
