---
id: RR-VDE7CV
type: review-response
title: SPA redirect drops the list scope
finding: The SPA redirect kept only the world from route.query, so a successor reached from a list lost its prev/next navigation.
severity: minor
resolution: A successor of the same type keeps the whole query, since it belongs to the same list. A successor of another type keeps only the world, as the owner redirect does, because it is outside that list. The owner test file asserts the kept query.
status: addressed
---
