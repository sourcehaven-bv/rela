---
id: RR-ALZFIJ
type: review-response
title: Ephemeral ring can be monopolised
finding: With ring 64 and per-identity cap 32, two refreshing identities hold a channel ring; 128 identities can occupy every ephemeral channel in the process.
severity: minor
resolution: One identity may hold at most 8 signals per ring and signals in at most 4 channels.
status: addressed
---
