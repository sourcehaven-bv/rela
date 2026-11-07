---
id: RR-OAPQ1HH
type: review-response
title: Purge can race a seal between key and index writes
finding: A purge that ran between the key write and the index write would miss the new key.
severity: significant
resolution: seal writes the key before the index, and a purge writes a new generation that seal binds into the sealed state.
status: addressed
---
