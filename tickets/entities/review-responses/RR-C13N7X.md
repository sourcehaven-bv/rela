---
id: RR-C13N7X
type: review-response
title: Validator does not check template value types
finding: Text templates and literals produce strings for integer/date/enum/boolean fields; a recipe with no {{item.text}} is accepted.
severity: minor
resolution: The validator now checks each field template against the target type (text fits string, rrule, custom string types, or an enum when it is a literal value) and requires some field to use {{item.text}}. Pinned by four validation cases.
status: addressed
---
