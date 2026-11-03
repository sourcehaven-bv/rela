---
id: RR-9QMHT8U
type: review-response
title: Form selection sort differs from the server
finding: localeCompare with numeric is locale-aware and mostly case-insensitive; the server uses byte-wise natural sort, so the dialog can pick a different form than the create page.
severity: minor
resolution: createFormFor sorts form ids with naturalCompare, a port of internal/natsort, with a matching test table.
status: addressed
---
