---
id: RR-COK0GT
type: review-response
title: CurrentUserBindings allocated a map and two closures on every affordance grant evaluation
finding: newBindings runs per grant per role per entity on the list-render read path and now called predicatefns.CurrentUserBindings(identity) each time — a fresh map plus two closures for a value constant across the whole bindingContext.
severity: minor
resolution: bindingContext gained a userFuncs field built once in bindingFor from bc.identity(); newBindings binds from it.
status: addressed
---
