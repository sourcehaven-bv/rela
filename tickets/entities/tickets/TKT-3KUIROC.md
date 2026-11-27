---
id: TKT-3KUIROC
type: ticket
title: Extract SSE and git handlers from dataentry.App
kind: chore
started: "2026-11-27"
completed: "2026-11-27"
status: done
---

## Description
Move the SSE and git handlers off App into sseHandler and gitHandler. App pins
80 to 72 methods.
