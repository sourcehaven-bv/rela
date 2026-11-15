---
id: RR-R6J1M0F
type: review-response
title: Issue body cut can break markdown
finding: head -c can cut inside a code fence or a UTF-8 character.
severity: nit
resolution: 'Addressed: whole lines only (head -n 600) and an odd fence count is closed.'
status: addressed
---
