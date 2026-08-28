---
id: RR-DR-DOTCLAIMS
type: review-response
title: The ticket overstates the dotfile rule's coverage and its lack of false positives
finding: 'Two inaccurate claims. (1) Ticket says ''editor swap files -> 404'' are covered. Vim swap files
  for custom.css ARE .custom.css.swp (dot-prefixed, caught), but vim/Emacs BACKUP files are custom.css~
  and #custom.css# - NOT dot-prefixed, so they are served (as octet-stream, unknown extension), leaking
  a prior revision of operator code. (2) Ticket claims the rule ''has no legitimate false positives''.
  Counter-example: .well-known/ - an operator wanting to serve an ACME challenge or security.txt from
  custom/ is blocked. Obscure enough to accept, but it falsifies the absolute claim, and absolute claims
  in security comments are how the next person justifies removing the check. Separately the rule catches
  none of: notes.md, backup.sql, id_rsa, credentials.json, custom.css.orig - it is a filename-shape heuristic
  standing in for a sensitivity classifier, and the two only overlap by convention.'
severity: significant
status: addressed
resolution: 'Folded into TKT-IWMETE and PLAN-6VVJJZ before implementation. Correct both sentences on the
  ticket: name ~ and #file# backups as NOT covered, and soften to ''no false positives we expect in practice''
  with .well-known noted. The rule stays - it is still strictly better than the extension map at .env.backup
  - but its documented scope must match its actual scope.'
---

Raised by `/design-review` of TKT-IWMETE before implementation.
