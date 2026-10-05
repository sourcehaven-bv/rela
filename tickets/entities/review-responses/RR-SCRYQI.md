---
id: RR-SCRYQI
type: review-response
title: DenyReader.ResolveHeaders fails silently
finding: It returned nil without a log and its comment described one caller.
severity: minor
resolution: It logs the refusal with slog.Warn and the comment states the contract.
status: addressed
---
