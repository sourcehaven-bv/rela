---
id: RR-YD4MH2
type: review-response
title: Grouped-list busy test does not pin "every page"
finding: EntityList.busy.test.ts:167 "grouped list sends busy on each page" mocks listAllEntities and only checks the params handed to it once. AC 6 says each page carries busy; that depends on listAllEntities forwarding params to every page request, which this test cannot see. Either add a listAllEntities unit case that asserts busy on page 2 and later, or rename the test to what it checks ("grouped loader receives busy").
severity: minor
resolution: ADDRESSED Added listAllEntities case "forwards busy and list_id to every page" (three pages) and renamed the EntityList test to "grouped loader receives busy".
status: addressed
---
