---
id: RR-LPEMA
type: review-response
title: 'App construction: two-step wiring of documentService'
finding: internal/dataentry/app.go:302-321. scriptEngine built first, then App constructed without documents, then app.documents = newDocumentService(...) because luaWriteDeps is a method on App. Temporal coupling; future constructors must remember step 2.
severity: nit
reason: Alternatives (passing *App into the service, splitting lua.WriteDeps construction out of App) are larger surgeries that don't return proportional value. The inline comment at the construction site documents the ordering explicitly.
status: wont-fix
---

From go-architect review finding #8.

Won't fix for this ticket: the alternative designs (pass *App into service,
split lua.WriteDeps construction out of App) are bigger surgeries that don't
return proportional value. The comment at the construction site makes the
ordering explicit.
