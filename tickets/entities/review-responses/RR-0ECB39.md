---
id: RR-0ECB39
type: review-response
title: Stale lowercase selectCommandAuthorizer in comments and a runtime error string
finding: |-
    Four references to `selectCommandAuthorizer` where the exported symbol is `SelectCommandAuthorizer` (internal/dataentry/app.go:890, :893, :1152 and command_handler.go:40). The one at app.go:893 is inside the error NewApp returns when the authorizer is nil, so it points an operator whose server just failed to start at a function they cannot find.

    The same error string also told callers to use `ungatedAuthorizer{}`, which is unexported and therefore unreachable from any external wiring site — both identifiers in an operator-facing diagnostic were unusable.
severity: minor
resolution: Capitalized all four references and changed the error string to name UngatedCommandAuthorizer(), the exported helper that external cmd entry points actually call. The nil-authorizer error now names two identifiers a caller can really use.
status: addressed
---
