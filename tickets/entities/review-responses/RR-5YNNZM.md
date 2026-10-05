---
id: RR-5YNNZM
type: review-response
title: Leftover undeclared face blocks the family delete
finding: A stored face the policy never grants (for example a face removed from the schema) makes the bare-id delete fail for everyone but an admin who holds it.
severity: minor
resolution: This is the fail-closed behaviour the bug asks for. The denial tests now assert which face was denied so the refusal names the face the role lacks.
status: addressed
---
