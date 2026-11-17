---
id: RR-UWUI00L
type: review-response
title: Required properties of a created type could be null
finding: The create schema let the model leave a required property empty.
severity: significant
resolution: Required properties without a default are non-nullable in the schema, refused as null on parse, and config requires them in properties.
status: addressed
---
