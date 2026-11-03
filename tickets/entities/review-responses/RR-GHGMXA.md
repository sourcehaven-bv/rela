---
id: RR-GHGMXA
type: review-response
title: Widget mapping duplicates constants and lacks file
finding: ResolveWidgetFromType returns literals duplicating dataentryconfig.Widget* and has no file case (RR-Z0GGTO).
severity: nit
reason: Predates this PR and the move does not change it; dataentryconfig imports metamodel, so using its constants needs its own change. Left for the next ticket that touches the widget mapping.
status: wont-fix
---
