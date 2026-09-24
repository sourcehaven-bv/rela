---
id: RR-HR17TV
type: review-response
title: Statement counter rationale wrong
finding: The real gap is that storetest.Counting does not forward GraphQueryHeaders or CountMatched.
severity: minor
resolution: Add the two forwarders to Counting instead of an exported statement counter.
status: addressed
---
