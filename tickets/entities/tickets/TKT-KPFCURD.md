---
id: TKT-KPFCURD
type: ticket
title: 'rela test: self-explaining failures and executable documentation from traces'
kind: enhancement
priority: medium
effort: m
status: backlog
---

## Description

On failure, print the trace that led to it, with the deciding rule and its file
and line ("denied: role gast has no update on incident; nearest grant
acl.yaml:42 requires owner"). Render passing test traces as readable scenarios
or sequence diagrams that serve as up-to-date documentation of the app. Builds
on the trace ticket.
