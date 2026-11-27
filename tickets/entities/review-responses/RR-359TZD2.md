---
id: RR-359TZD2
type: review-response
title: Guard denial unaudited on trigger write
finding: 'A transition-guard denial from a relation trigger returned a bare GuardError: no 403 and no denied-write audit row.'
severity: significant
resolution: 'Routed through mapTransitionError: ForbiddenError (transition-guard) plus one denied-write row; trigger and patch tests pin it.'
status: addressed
---
