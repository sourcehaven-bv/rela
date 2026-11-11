---
id: RR-XDYSD18
type: review-response
title: Triggers and reminders can target a type with a required file
finding: A trigger or reminder posts no files, so a type with a required file property would refuse every post at runtime.
severity: minor
resolution: literalPost refuses such a config at load. Documented in the realtime guide; tested in TestParse_Invalid.
status: addressed
---
