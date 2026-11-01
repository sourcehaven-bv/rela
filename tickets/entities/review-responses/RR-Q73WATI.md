---
id: RR-Q73WATI
type: review-response
title: Math.random in backoff breaks replay
finding: Backoff durations decide the order of the clients' retries, so a seed does not reproduce. Stub Math.random.
severity: significant
resolution: Math.random is stubbed to 0.5 in every run.
status: addressed
---
