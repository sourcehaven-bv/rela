---
id: RR-JAGVXB
type: review-response
title: A damaged meta.yaml logs a warning on every timer round
finding: filechannels visitTypesAt (file.go, skipDamaged branch) calls slog.Warn on every ListSessionsDue. That is at least once a minute, and once a second while rounds report More, for as long as the file stays damaged. Rate-limit the warning per key (for example once per key per process, or per TimerRecompute).
severity: nit
resolution: Damaged meta.yaml and SQL row warnings log at most once per channel per 10 minutes via channels.DamageWarner; TestDamageWarner.
status: addressed
---
