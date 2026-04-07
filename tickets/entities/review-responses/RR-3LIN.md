---
id: RR-3LIN
type: review-response
title: Config validation doesn't check file existence
finding: Typos in data-entry.yaml cause cryptic 500s at runtime instead of startup errors. Check file exists at config load. Also decide behavior when actions/ dir doesn't exist at all.
severity: significant
resolution: Config validation checks script file exists via os.OpenRoot at load time. Startup error if missing. Tests cover dir-missing cases.
status: addressed
---
