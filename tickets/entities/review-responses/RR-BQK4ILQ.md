---
id: RR-BQK4ILQ
type: review-response
title: typePages changes Limit between cursor calls
finding: The ListEntitiesPage contract said to keep other query fields identical on resume.
severity: minor
resolution: Every backend encodes the cursor as the last state key. The contract now allows Limit to change, and storetest EntitiesWalkWithGrowingLimit pins it on every backend.
status: addressed
---
