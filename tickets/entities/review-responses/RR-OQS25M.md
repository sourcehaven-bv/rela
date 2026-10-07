---
id: RR-OQS25M
type: review-response
title: VersionByTag returns an ordinal then re-reads
finding: A purge between the ordinal read and GetVersion shifts ordinals and returns the wrong merge base.
severity: significant
resolution: VersionByTag returns the snapshot from one read-only transaction on both backends.
status: addressed
---
