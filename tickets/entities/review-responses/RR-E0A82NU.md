---
id: RR-E0A82NU
type: review-response
title: Manager test covers minting only
finding: idprefix_case_test.go did not cover the widened manual-id path through the manager.
severity: minor
resolution: TestCreate_ManualIDPrefixFoldsCase creates a manual doc- entity with doc-intro and DOC-setup and expects note-x to be refused.
status: addressed
---
