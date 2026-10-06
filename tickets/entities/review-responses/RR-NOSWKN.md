---
id: RR-NOSWKN
type: review-response
title: In-Tx face re-authorization branches were untested
finding: Neither the in-Tx re-read of the family nor the check of what the store removed had a test. A regression that dropped either would pass the suite.
severity: significant
resolution: Added TestFamilyDelete_FaceAddedDuringDeleteIsAuthorized with a store wrapper that adds a face before the Tx and just before the store delete. It runs on memstore/fsstore/sqlite/postgres. A mutation that skips the post-delete check fails the before-store-delete cases.
status: addressed
---
