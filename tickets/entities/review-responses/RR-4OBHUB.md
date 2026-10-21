---
id: RR-4OBHUB
type: review-response
title: Windows zip archives untested
finding: syft was only tried on a tar.gz; the stdlib guard also runs on the Windows zips.
severity: minor
resolution: 'Dry run of the sbom step against all 18 v26.10.14 archives, zips included: all pass the stdlib guard.'
status: addressed
---
