---
id: RR-C5ALAE
type: review-response
title: 'Design: scan_sockets: [] and null are silently accepted'
finding: validateAttachments checks len(RemovedScanSockets) == 0, so an empty list or a null value passes, breaking the 'a schema that still sets it fails' contract. Keeping a dead exported field on AttachmentsConfig is clumsy; detect key presence instead.
severity: minor
resolution: AttachmentsConfig.UnmarshalYAML records presence of the scan_sockets key (unexported scanSocketsSet), so a list, [] and a bare key all fail with the pointer to RELA_SANDBOX_READ_PATHS. The exported RemovedScanSockets field is gone. TestParse_ScanSocketsRemoved covers all three values.
status: addressed
---
