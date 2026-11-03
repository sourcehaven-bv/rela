---
id: RR-YVYZ6SK
type: review-response
title: Damaged segment record in GetMessages is not reported through DamageWarner
finding: A damaged meta.yaml goes to d.damaged.Warn, but a damaged record inside a segment only reaches slog via fold. AC 2 says both are reported through the backend's DamageWarner.
severity: minor
resolution: GetMessages folds segments with foldWith and reports damaged records once per channel through d.damaged.Warn (filechannels/getmessages.go). The store doc says so.
status: addressed
---
