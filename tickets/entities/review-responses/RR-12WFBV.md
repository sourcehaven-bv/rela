---
id: RR-12WFBV
type: review-response
title: Ref text codec edge cases unspecified
finding: Behaviour of empty input and ID@ and @face and marshalling a zero Ref is not defined
severity: minor
resolution: 'Design section 8.8 (PR 1): ParseRef/UnmarshalText reject empty input and @face and ID@; MarshalText on the zero Ref errors; table-driven tests plus a JSON map-key round trip.'
status: addressed
---

**Where:** design section 1.

The Ref codec leaves edge cases open: `UnmarshalText("")`, `"ID@"`, `"@face"`,
and `MarshalText` on the zero `Ref`. A zero Ref used as a JSON map key would
serialize as `""` and parse back as an error or as a different key.
