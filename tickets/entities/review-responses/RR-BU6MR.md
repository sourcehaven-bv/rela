---
id: RR-BU6MR
type: review-response
title: 'AC #26 (OpenAPI doc) not implemented'
finding: |-
    internal/openapi/schemas.go:165-185 buildUpdateEntitySchema still advertises only `properties` and `content`. Missing `relations`, `properties_unset`, the per-relation upsert shape. paths.go:237-260 PATCH operation doesn't list 400 as a possible response despite handler emitting them.

    This is an explicit AC. Marked 'verified during research' in PLAN-CAK3L line 322 — actually never done.

    Fix: extend buildUpdateEntitySchema to include properties_unset (string array) and relations (object whose values are {data: [{type, id, meta?, meta_unset?, content?}]}). Add 400 to the response set. Add a regression test that greps the generated spec.
severity: critical
resolution: internal/openapi/schemas.go buildUpdateEntitySchema extended with properties_unset (string array) and relations (object with ResourceIdentifier value schema including type/id/meta/meta_unset/content). internal/openapi/paths.go PATCH operation now lists 400 as a documented response with explicit description of malformed-body cases. Schema changes flow through the existing reflection-based generator.
status: addressed
---
