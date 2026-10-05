---
id: RR-UITNVE
type: review-response
title: Ref.MarshalText does not check canonical form
finding: MarshalText validated the id and face separately and discarded ParseFace's result, so its doc claim of refusing a non-canonical face held only while canonicalization is the identity.
severity: minor
resolution: MarshalText now parses its own String() and requires the result to equal the receiver, so the round-trip rule lives in ParseRef alone.
status: addressed
---
