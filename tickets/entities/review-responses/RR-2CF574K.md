---
id: RR-2CF574K
type: review-response
title: Hostile remote can harvest local addresses through split bounds
finding: Bucket bounds are local addresses; a remote that always reports a mismatching large count keeps the client splitting and collects local addresses the remote principal may not know.
severity: significant
resolution: A span with at most 256 local elements is listed and never split; the remote counts of a split's parts must add up to its count of the whole or the run fails with ErrProtocol (TestDiff_CountsMustAddUp, TestDiff_SmallLocalSpanIsNotSplit); documented in doc.go and the CLI reference.
status: addressed
---
