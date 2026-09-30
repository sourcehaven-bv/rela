---
id: RR-0RU1H1
type: review-response
title: Comments and CLAUDE.md said guards do not apply on restore
finding: recreate.go, the archguard advice and internal/entitymanager/CLAUDE.md described the exemption as covering transition guards, which no longer matches the code.
severity: minor
resolution: 'Reworded all three: only the entry rule is skipped; guards bind; when: is not evaluated; automations do not run.'
status: addressed
---
