---
id: RR-3C82L
type: review-response
title: ValidationErrorInvalidValue missing from IsSoft — enum/regex/date violations stay hard 422
finding: 'IsSoft switch lists Required + InvalidType but omits InvalidValue. Verified: InvalidValue fires for enum-not-in-allowlist, regex mismatch, bad date format, RRULE parse failure, bad-integer-string, bad-boolean-string, custom-type validations (lines 180, 193, 213 in validation.go). All textbook DEC-HWZHA soft conditions a hand-editor produces. Recommendation: add ValidationErrorInvalidValue to IsSoft. Add property_value_invalid warning code. Add ACs: PATCH enum-mismatch → 200 + warning, PATCH bad-date → 200 + warning. Without this the ticket only does half the job. From design-review F2.'
severity: critical
resolution: 'ValidationErrorInvalidValue added to IsSoft() switch in Layer 0. New ACs cover: AC3 (enum value not in allowlist), AC4 (bad date), AC5 (bad RRULE). New warning code property_value_invalid in mapping table. Plan now correctly softens all hand-editor-producible property-level errors.'
status: addressed
---
