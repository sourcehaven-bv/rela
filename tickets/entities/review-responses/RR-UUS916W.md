---
id: RR-UUS916W
type: review-response
title: Remote memory bounded by element count only
finding: 1M elements of up to 4 KB addresses could retain about 4 GB.
severity: minor
resolution: Options.MaxRemoteBytes (default 256 MiB) caps kept address bytes. TestDiff_RemoteBytesBudget.
status: addressed
---
