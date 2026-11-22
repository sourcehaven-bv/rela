---
id: RR-OI85UDC
type: review-response
title: "The shell decodes the whole app block"
finding: "ShellBootFrom now decodes app: into AppConfig, so a malformed key elsewhere in app: drops the shell values."
severity: significant
resolution: "Won't fix: the full config load fails on the same YAML and reports it; the shell then shows the build's name, as before for a broken file."
reason: "A malformed key in app: also fails the full config load, which reports it at startup. The shell then falls back to the name the build carries, which is what a broken data-entry.yaml showed before this change."
status: wont-fix
---
