---
id: RR-DW5YD5
type: review-response
title: retention_active title reports an end time up to 1s early
finding: internal/realtime/lifecycle.go:170 formats `until` with `time.RFC3339`, but ClosedAt is stored at microsecond precision (channels.NormTime). A client that retries at the advertised instant gets 409 again. Round `until` up to the next second for the title, or use RFC3339Nano. The tests miss this because the clock is whole-second; add a row with a fractional ClosedAt.
severity: minor
resolution: 'retentionCheck rounds a fractional end time up to the next whole second for the title. New subtest "fractional close time": the title shows the rounded second, and a delete at exactly that time succeeds.'
status: addressed
---
