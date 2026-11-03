---
id: RR-FB3EZI
type: review-response
title: Fork PRs would fail on the SARIF upload
finding: A fork PR's token cannot write security events, and the upload step ran with always().
severity: significant
resolution: The upload step runs only on merge_group or when the PR head repo is this repository.
status: addressed
---
