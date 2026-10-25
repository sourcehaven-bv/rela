---
id: RR-IJYDME
type: review-response
title: Failed timer writes are logged twice, and Stranded can double count in one round
finding: Timers.write logs each failed write at Warn (timers.go:474) and joins the error. realtimeWorker.timerRound then logs the joined error at Warn again (realtime_worker.go:519). TimerReport.Stranded is incremented in warnStranded during a recompute (timers.go:338) and again in fireDue for a listed stranded row (timers.go:407-409), so a round that does both reports the same session twice. Log once (drop the per-row log or the joined one), and count stranded rows in one place.
severity: nit
resolution: A failed timer write is logged once by the worker, naming the channel; stranded counts once per round (TestTimerRound_StrandCountsOnce).
status: addressed
---
