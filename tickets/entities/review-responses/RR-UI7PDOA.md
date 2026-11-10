---
id: RR-UI7PDOA
type: review-response
title: Safe defaults do not reach data-entry app runtimes
finding: dataentry luaWriteDeps and wireValidation build lua.ReadDeps without Seams or Host, so an app mounted on an in-memory world would read real secrets/ai.yaml and send real mail.
severity: significant
resolution: 'NewInMemory and WithClock godoc now state the boundary: only runtimes built from the services'' deps get the seams; mounting the data-entry app on a world needs its runtimes wired from the world''s deps first. No world mounts the app in this PR.'
status: addressed
---
