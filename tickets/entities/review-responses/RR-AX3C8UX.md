---
id: RR-AX3C8UX
type: review-response
title: Thread query skipped the thread index
finding: The OR form of the thread filter walked every message of the channel on SQLite (and Postgres generic plans).
severity: significant
resolution: Thread page is now a UNION ALL of the parent by key and the replies through channel_messages_thread_idx; EXPLAIN tests on sqlite and postgres pin both arms.
status: addressed
---
