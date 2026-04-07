---
id: RR-UZEX
type: review-response
title: rela.output collision silently ignored
finding: If a script calls rela.output in action context, it's silently dropped. This causes confusing 'works in scripts but not actions' bug reports. Log warning or raise error.
severity: significant
resolution: Runtime has isAction flag. rela.output in action context logs warning to server stdout, output dropped from response.
status: addressed
---
