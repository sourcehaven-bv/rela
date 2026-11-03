---
id: RR-6CH6B8
type: review-response
title: ConflictsView strings are not translated
finding: Strings such as Failed to load conflicts and Retry are hardcoded English while the new message uses i18n.
severity: nit
reason: Pre-existing and outside this bug; the file is held to its bare-string budget and can be translated in a separate change.
status: deferred
---
