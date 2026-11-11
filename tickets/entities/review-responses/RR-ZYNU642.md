---
id: RR-ZYNU642
type: review-response
title: SQL schema keying and Raw coverage
finding: Keying by anchor would break rekey/tombstone; Raw() must include the table; dead tuples hold bytes.
severity: minor
resolution: channel_files keyed by channel_id FK ON DELETE CASCADE + seq + file id; added to sqlchannels Raw; AC6 reworded to live rows.
status: addressed
---
