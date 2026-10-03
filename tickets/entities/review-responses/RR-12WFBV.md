---
id: RR-12WFBV
type: review-response
title: Ref text codec edge cases unspecified
finding: Behaviour of empty input and ID@ and @face and marshalling a zero Ref is not defined
severity: minor
resolution: 'Already covered by merged PR 1 (#1712): ParseRef rejects empty input and @face and ID@; MarshalText refuses a non-canonical or zero Ref; ref_test.go pins each case and the JSON round trip. Noted in design section 8.8.'
status: addressed
---

**Where:** design section 1.

The Ref codec leaves edge cases open: `UnmarshalText("")`, `"ID@"`, `"@face"`,
and `MarshalText` on the zero `Ref`. A zero Ref used as a JSON map key would
serialize as `""` and parse back as an error or as a different key.
