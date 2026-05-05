---
id: RR-DDL04
type: review-response
title: Description-text test uses substring match
finding: TestUpdateEntityToolDescriptionMentionsNullDelete checks contains('null') which would also match 'nullify', 'unnull', etc. Match the canonical phrase ('set a property to null') or use a whole-word match for safety.
severity: nit
resolution: Tightened the test to match the canonical phrase 'set a property to null' instead of just 'null'. Substring is now specific enough that 'nullify' / 'unnull' / etc. wouldn't match.
status: addressed
---
