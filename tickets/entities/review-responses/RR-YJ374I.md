---
id: RR-YJ374I
type: review-response
title: Start/Stop lifecycle duplicated in both stores
finding: About 40 identical lines in pgstore and sqlitestore sweep.go.
severity: minor
resolution: Extracted internal/store/bgloop (100% covered, arch-lint registered); both sweeps wrap it.
status: addressed
---
