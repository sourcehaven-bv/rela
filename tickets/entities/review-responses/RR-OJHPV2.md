---
id: RR-OJHPV2
type: review-response
title: DenyReader and zero ReadQueryResult behaviour contradict existing conventions
finding: 'The plan says DenyReader returns none. DenyReader''s convention is ErrReaderUnavailable, so a refused reader is diagnosable and never read as empty. The negative test "MatchingRefs with a zero ReadQueryResult: error" will not hold through composeReadScope, which returns ok=false (fallback) when rqr.Query is nil. Only scopedHeaders errors. Fix: return ErrReaderUnavailable from the deny variant, and either handle the zero result explicitly as an error or change the test to assert the gated fallback.'
severity: minor
resolution: Refusing HolderLookup returns ErrReaderUnavailable. A zero ReadQueryResult is handled explicitly as an error, as scopedHeaders does, with TestHolderLookup_ZeroResultErrors.
status: addressed
---
