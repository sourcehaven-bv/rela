---
id: RR-62RAH37
type: review-response
title: Action targets validated only at runtime
finding: A config mistake on an action target would show up as a permanent failure on a live run after its outputs were written.
severity: significant
resolution: 'Plan: buildFlowEngine checks every action target against data-entry.yaml and fails the build; the runtime check stays for reloads.'
status: addressed
---
