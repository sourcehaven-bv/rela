---
id: RR-VWQ38Q
type: review-response
title: scan_sockets detection misses YAML merge keys
finding: 'attachments: {<<: {scan_sockets: [/a]}, scan_cmd: [a]} parses without error because UnmarshalYAML only checks direct mapping keys. Follow << values (mapping, alias, sequence).'
severity: minor
resolution: hasKey follows merge keys (mapping, alias, sequence) when detecting scan_sockets. TestParse_ScanSocketsRemoved covers an inline merge mapping; anchors elsewhere in the schema are not possible because unknown top-level keys are rejected.
status: addressed
---
