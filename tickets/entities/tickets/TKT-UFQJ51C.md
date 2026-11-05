---
id: TKT-UFQJ51C
type: ticket
title: TUI spike on the resolved view builders
kind: chore
priority: low
effort: m
tags: needs-design
status: backlog
---

## Description

Build a throwaway TUI (for example with bubbletea) that calls the resolved view,
kanban and form builders in-process. It does not ship. Record every place where
the contract still assumes a browser, and file follow-up tickets.

Part of FEAT-QRI3CFR (step 4).
