---
id: RR-3DJJF
type: review-response
title: propertyType:string is too loose
finding: Metamodel has a finite set (string|boolean|date|integer|rrule|markdown|enum). Typing as string defers a runtime 'unknown propertyType' branch into every widget.
severity: minor
resolution: 'Plan revised: PropertyType discriminated union added (''string''|''boolean''|''date''|''integer''|''rrule''|''markdown''|''enum''). Used in WidgetProps and registry. New metamodel types will be type-checked into widget updates.'
status: addressed
---
