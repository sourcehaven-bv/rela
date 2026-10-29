---
id: RR-MK9Z0I
type: review-response
title: OpenAPI list-property schema is untested, although AC 3 names the goldens
finding: 'internal/openapi/channels.go:590 propSchema adds items, maxItems and uniqueItems for a list property in the post body. No golden or unit test has a list property: grep finds 0 uniqueItems and 0 maxItems in decl-jwt/openapi.authenticated.golden, and openapi/channels_test.go:172 checks only the scalar text type. The notes report AC 3 as covered by the OpenAPI goldens, which is not true. A regression in the recursion, for example Items dropped or MaxLength not carried to the items, would ship silently to API clients. Fix: add a case to openapi/channels_test.go next to :172 that runs propsSchema over a type with options: {type: string, list: true, max: 80, max_items: 4} and asserts {type: array, items: {type: string, maxLength: 80}, maxItems: 4, uniqueItems: true}, or add a list property to the golden fixture''s realtime.yaml.'
severity: significant
resolution: 'Added TestChannelPaths_ListProperty in internal/openapi/channels_test.go: a list property {type: string, list: true, max: 80, max_items: 4} goes through realtimecfg.Parse and Generate, and the test asserts the post body schema {type: array, items: {type: string, maxLength: 80}, maxItems: 4, uniqueItems: true}.'
status: addressed
---
