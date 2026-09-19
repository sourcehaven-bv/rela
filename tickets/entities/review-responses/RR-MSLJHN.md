---
id: RR-MSLJHN
type: review-response
title: 'Named integration test passes vacuously: tickets/ declares zero faces'
finding: 'The plan named the tickets project''s own 14 gates as the integration test: ''rela validate over tickets/ must report identically before and after''. But tickets/schema.yaml declares ZERO faces (grep -c ''faces:'' -> 0) and no scope: content relations, so it exercises only the faceless, one-edge-per-triple case — exactly the case where the multiplicity hazard cannot bite and where the ''face behaviour unchanged'' criterion is satisfied by construction rather than verification. That is the ''clean run over data it never looked at'' failure this codebase repeatedly warns about (metamodel/types.go:96-112, loader.go validateValidationFaces).'
severity: critical
resolution: 'Confirmed by direct count: zero faces in tickets/schema.yaml. tickets/ is retained as a regression check for the faceless path, but demoted from ''the integration test''. The face and multiplicity criteria now require a purpose-built fixture with a faced type and a face-tailed relation; internal/validator/faces_test.go already exists and is named as the place to extend.'
status: addressed
---
