---
id: RR-N1B15K1
type: review-response
title: Unbounded entity values are sent to the model
finding: A long property value was sent whole, raising cost and injection surface.
severity: minor
resolution: Values are capped at 2000 characters and the record names cut fields; tested.
status: addressed
---
