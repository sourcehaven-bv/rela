---
id: TKT-3KUIROC
type: ticket
title: Extract SSE and git handlers from dataentry.App
kind: chore
status: backlog
---

## Description
Move the SSE and git handlers off App into sseHandler and gitHandler. App pins
80 to 72 methods.
