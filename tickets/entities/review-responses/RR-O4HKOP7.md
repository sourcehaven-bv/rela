---
id: RR-O4HKOP7
type: review-response
title: Concurrent writes can repeat a row across windows
finding: Windows are separate statements, so a row inserted before the offset between two windows can be returned twice.
severity: minor
resolution: firstNByTypes keeps each entity id once.
status: addressed
---
