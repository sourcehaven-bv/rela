---
id: RR-97YYCO
type: review-response
title: Record times are not normalized or encoded as in slice 3
finding: 'Plan line 482 has ApplySchedule check the records but not normalize them. Restarts are passed through NormTime (internal/channels/session.go:65-71) and stored as JSON microseconds by encodeRestarts. The reminder CAS compares the listed Since with the committed one, so a backend that round-trips Since with a different precision or location (YAML in meta.yaml, JSON text in SQL) makes the CAS fail on every round. That fails closed, so no reminder is ever posted. Plan change: ApplySchedule applies NormTime to Since (and to Last, see the first finding). sqlchannels encodes the records as microseconds like encodeRestarts. The CAS compares with time.Equal. testSessionRemindersStored asserts equality after the round trip on all four backends.'
severity: minor
resolution: 'Addressed: NormReminder applies NormTime to Since and Last on every write. sqlchannels encodes them as microseconds like encodeRestarts, and the CAS uses time.Equal. A round-trip test runs on four backends.'
status: addressed
---
