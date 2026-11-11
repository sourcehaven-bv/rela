---
id: RR-47CU37H
type: review-response
title: Headless builds compiled only for linux in CI
finding: The release ships headless binaries for darwin and windows, but the CI job built linux only.
severity: significant
resolution: The headless job builds both variants for linux, darwin and windows.
status: addressed
---
