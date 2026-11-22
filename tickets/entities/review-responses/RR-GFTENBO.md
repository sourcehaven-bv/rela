---
id: RR-GFTENBO
type: review-response
title: Tab shows the raw entity id while loading
finding: entryTitle falls back to the id, so the tab read the id on load and on a failure.
severity: minor
resolution: The tab gets no page title until the entity has loaded.
status: addressed
---
