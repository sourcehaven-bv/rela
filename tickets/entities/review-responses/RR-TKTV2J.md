---
id: RR-TKTV2J
type: review-response
title: Card rendering for several busy channels on one anchor is unspecified
finding: AC 2 returns several entries for a cardinality-many type, but AC 9 and step 9 describe one indicator with one state label. The plan should say what the card shows for two or more entries (for example, the first label plus "+1", with every entry in the accessible text) and add a cardBusy test case for it.
severity: minor
resolution: Two or more entries show the first label plus "+N", with every entry in the accessible name; AC 9 adds a cardBusy test case for it.
status: addressed
---
