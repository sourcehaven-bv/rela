---
id: RR-YEX0H73
type: review-response
title: Duplicate row-key comparers and subtest names
finding: compareRowKey exists in three packages and a ScaleFree subtest name repeated.
severity: nit
resolution: Subtests are named alice, alice filtered, bob. The comparers stay per package (one line each; a shared helper would add an import for no gain).
status: addressed
---
