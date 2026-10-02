---
id: RR-3JR09P
type: review-response
title: Enum label and style lookup changed from propType to property name
finding: Badge used to get :property=cell.propType; it now gets the column property plus entity type through SelectWidget.
severity: minor
resolution: 'Kept: resolving labels and styles by property (with entity type) is the documented Badge contract; noted in the commit message.'
status: addressed
---
