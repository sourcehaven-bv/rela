---
id: RR-8I32H8V
type: review-response
title: Rows carry no _versions so cell saves are unguarded
finding: 'The plan''s 412 three-way merge claim is wrong: rows lack _versions (TKT-MK9NJB). Either add _versions to rows or state saves are unguarded until the first save.'
severity: significant
resolution: Rows stay unguarded until the first save returns tokens, as list rows (TKT-MK9NJB). Stated in the plan and in frontend/CLAUDE.md.
status: addressed
---
