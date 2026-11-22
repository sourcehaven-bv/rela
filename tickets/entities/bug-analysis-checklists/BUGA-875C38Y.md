---
id: BUGA-875C38Y
type: bug-analysis-checklist
title: 'Analysis: Long property values on phones; HTML answers taken as API data'
started: "2026-11-22"
completed: "2026-11-22"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Reproduction

- [x] Bug reproduced locally (a new e2e test on the phone viewport set a 54-character assignee on TASK-001: its value ran past the "Task" and "Narrow fields" sections; a unit test sent a 200 HTML page through the API client and got the string back as data)
- [x] Minimal reproduction steps documented (give a text property a long value without spaces and open the entity at 375px; answer an API call with a 200 text/html page)
- [x] Environment/conditions noted (phone width or a narrow span; a proxy in front of rela that answers with its own page)

## Root Cause

- [x] Immediate cause identified (why1)
- [x] Contributing factors found (why2-3)
- [x] Systemic cause explored (why4-5)

## Fix Planning

- [x] Fix approach determined (overflow-wrap: anywhere on the shared property value and row classes in properties-list.css; the client response interceptor rejects an HTML body on a JSON call)
- [x] Regression test planned (mobile-layout.spec.ts for the wrap; htmlResponse.test.ts through the real axios instance)
- [x] Related areas checked for similar issues (SidePanel shares .property-item and gets the same rule; long values in property-long already wrapped; blob downloads through getAxios are left alone by responseType)
