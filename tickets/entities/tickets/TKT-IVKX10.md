---
id: TKT-IVKX10
type: ticket
title: Archplan security declarations and a seqtrace routes suite
kind: chore
status: review
---

## Description

Declare trust zones, guards, sinks and entry-point auth for archplan as marker
comments and GLOSSARY.md front matter, and add `SEQTRACE_SUITE=routes` to the
seqtrace demo: one request per route family, so traces show the identity and
access checks per route. The archplan security detectors read both.
