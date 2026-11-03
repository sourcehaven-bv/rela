---
id: RR-A13XZ1
type: review-response
title: State captured more than once per operation
finding: AttachRender took a snapshot and the document gates loaded State again.
severity: minor
resolution: standaloneDocumentGates and anchoredDocumentGates take the caller's snapshot. The list export pipeline still loads its own config, as the HTTP export route does; threading the snapshot through that shared pipeline is out of scope.
status: addressed
---
