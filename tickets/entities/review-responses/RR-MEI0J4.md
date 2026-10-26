---
id: RR-MEI0J4
type: review-response
title: Grant check refuses every family-anchored set over mixed faced/unfaced attach_to
finding: 'internal/realtimecfg/grants.go:128 checks a family-anchored set at entity.Face(o.Face) on every attach_to type, including unfaced ones. setFace (outputs.go:157) accepts `face: draft` for attach_to [page, datalek], and at runtime managerOutputs.untypedRow (realtime_outputs.go) writes the unfaced row with no face. But the check asks Verb(update, datalek, "draft"), which no policy can grant: ValidateAgainstMetamodel refuses a face on an unfaced type. Reproduced with a go test -overlay probe: attach_to [page, datalek], set {face: draft}, bot granted update [page@draft, datalek] -> "system:bot cannot update datalek@draft". Because loading is strict, such a config can never load. Fix: in grantChecker.set, use the zero face for a type with no declared faces (faces = []entity.Face{""} when FaceOrderOf(typ) is empty). Add a TestRunAsGrantCheck row and an agreement row for mixed attach_to.'
severity: significant
resolution: The grant check uses the zero face for types without faces; a family-anchored set over page@draft and datalek now loads (new test).
status: addressed
---
