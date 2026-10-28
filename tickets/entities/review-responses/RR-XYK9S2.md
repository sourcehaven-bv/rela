---
id: RR-XYK9S2
type: review-response
title: AC 4 "spends no rate token" holds only for the pre-check path
finding: In Service.Post (internal/realtime/service.go:364-427) the limiter (line 386), checkMessageKeeping, s.record(c) and precheckOutputs all run before store.Append. A refusal by the store gate after a concurrent move has therefore already spent a token and may have written the type record. That is acceptable, but AC 4 and the security section state it without qualification, and the AC 5 concurrency test should not assert an untouched limiter for the losing post. Say "a post refused by the pre-check spends no token; a post refused by the store after a concurrent move spends one, like any failed append".
severity: minor
resolution: AC 4, the security section and the AC 5 test now say a pre-check refusal spends no token, while a store refusal after a concurrent move spends one like any failed append; the race test does not assert the loser's limiter.
status: addressed
---
