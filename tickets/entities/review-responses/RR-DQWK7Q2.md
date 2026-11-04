---
id: RR-DQWK7Q2
type: review-response
title: 15s owed-event wait can hide the replay seed
finding: endOnFailure false makes fast-check shrink; each failing rerun waits OWED_EVENT_MS in until; about 40 reruns reach the 600s vitest timeout before the seed is printed.
severity: significant
resolution: OWED_EVENT_MS lowered to 5s (a flush is 200ms and a live round 500ms) with a comment on why it stays short.
status: addressed
---
