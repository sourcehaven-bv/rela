---
id: RR-QS3N
type: review-response
title: 'Mention public API: inaccessible_reason is a bare string — type-safety dropped at the wire'
finding: 'internal/dataentry/mentions.go line 24 declares Mention.InaccessibleReason as `string`, even though entity.InaccessibleField.Reason is the typed `entity.InaccessibleReason` (an alias for string with documented constants like InaccessibleReasonGitCrypt). On the wire `inaccessible_reason` becomes free-form string; the SPA''s Mention.inaccessible_reason matches. This means: (a) the back-end loses type-checking on what reasons it produces — a typo at the call site won''t be caught; (b) the SPA''s `inaccessibleTooltipFor` (markdown.ts line 112) is the ONLY consumer that cares about specific values (''git-crypt''), and it''s a string literal compare — no shared enum, no exhaustiveness check. Suggestions: (1) declare `InaccessibleReason string` as the wire type (just JSON-encodes as the string anyway); (2) on the SPA side, type Mention.inaccessible_reason as a known union (''git-crypt'' | string-tag) so the tooltip helper gets an exhaustiveness signal when a new reason is added. Today the contract is also undocumented — no @stable comment on the Go struct, no link from the SPA interface to the Go truth. Add a comment to V1ViewResponse.Mentions noting the JSON shape is public API surface and changes are breaking.'
severity: minor
resolution: 'Documented stability contract on both Go (V1ViewResponse doc-comment) and TS (Mention interface) sides: inaccessible_reason is an opaque bare string; client must treat unknown reasons as opaque. The const set lives on Go side via entity.InaccessibleReason.'
status: addressed
---
