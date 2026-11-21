---
id: RR-VBFHW4K
type: review-response
title: Zero-value CardFigureSource panics
finding: CardFigureSource{} panics on c.app.State().
severity: nit
reason: The field is unexported and CardFigures rejects a nil App, so a zero value can only be built inside the package.
status: wont-fix
---
