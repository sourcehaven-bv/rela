---
id: RR-HM9XAW1
type: review-response
title: searchPage and searchPosition load services more than once
finding: structuredPlan loaded q.services() for the metamodel, then page/position and the fallback loaded it again; a reload in between could plan against one metamodel and read another store.
severity: minor
resolution: 'Addressed: searchPage and searchPosition load svc once and pass it to structuredPlan, page and position.'
status: addressed
---
