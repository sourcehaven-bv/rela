---
id: RR-KZI23C
type: review-response
title: Config loaded per BeforeDelete call
finding: blocking() loads the config per call, so a root plus owned entities can see different configs across a reload.
severity: nit
status: open
---
