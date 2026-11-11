---
id: RR-K09BZ2V
type: review-response
title: filechannels writes large files under the project-wide lock
finding: The filechannels lock covers every channel of the project; writing up to 40 MiB under it stalls all posts and polls.
severity: significant
resolution: Bytes are written to a staging directory outside the lock; under the lock only a rename happens. Stale staging entries are removed by TTL on the next Append.
status: addressed
---
