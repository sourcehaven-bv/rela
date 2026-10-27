---
id: RR-QQTRI8
type: review-response
title: Language can flip once after the first render
finding: app.language arrives with the schema, so the shell follows the browser until it loads.
severity: minor
reason: Holding the whole UI back on the config request costs more than one early switch. Documented as accepted in frontend/CLAUDE.md.
status: wont-fix
---
