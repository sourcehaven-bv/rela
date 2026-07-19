---
id: RR-9J59FE
type: review-response
title: 'Badge test used `entityType: ''ticket'' as never` cast'
finding: The disambiguation test passed a string entityType through a cast, exercising a path no real caller could use given the object-only prop type.
severity: minor
resolution: Cast removed after the prop was widened to string | EntityType; the string path is now a real, typed contract used by the widget call sites.
status: addressed
---
