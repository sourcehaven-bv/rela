---
id: RR-5M0X0M
type: review-response
title: max means two things on the wire and in the widget prop
finding: WidgetProps.max is documented as the attachment cap but now also carries an integer bound; /_schema emits max on integers.
severity: minor
resolution: 'WidgetProps.max doc now says only the file widget reads it. The shared wire key is deliberate: the brief named the schema keys min/max and max already meant the most; the wire mirrors the schema, documented on v1.PropertyDef.'
status: addressed
---
