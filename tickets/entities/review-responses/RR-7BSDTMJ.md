---
id: RR-7BSDTMJ
type: review-response
title: Conversion errors can hide in the collapsed section
finding: fieldErrors also targets fields inside the collapsed details element, so an error there is invisible. Errors also stay after the user fills the field.
severity: minor
resolution: The optional-fields details opens when one of its fields has an error, and editing a field clears its error.
status: addressed
---
