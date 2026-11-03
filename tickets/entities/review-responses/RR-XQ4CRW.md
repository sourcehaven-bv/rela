---
id: RR-XQ4CRW
type: review-response
title: _lease could leak through shared v1.Entity paths
finding: 'Security design review: views, history, includes, write responses and fieldversions tokens share the struct.'
severity: minor
resolution: 'Plan: only entity GET and list rows set _lease after fieldversions; test hidden fixture through other paths.'
status: addressed
---
