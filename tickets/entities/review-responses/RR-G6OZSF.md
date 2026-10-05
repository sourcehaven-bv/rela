---
id: RR-G6OZSF
type: review-response
title: Absence assertions can pass before render
finding: expectNoEdit/expectNoCopy and the control-card count(0) checks ran right after the heading and could pass before the action row or section rendered.
severity: significant
resolution: expectNoEdit and expectNoCopy first wait for the export menu in the action row; control-card absence checks first wait for the implemented-by section (FacesPage.waitForSection).
status: addressed
---

expectNoEdit/expectNoCopy and the control-card count(0) checks ran right after
the heading and could pass before the action row or section rendered.
