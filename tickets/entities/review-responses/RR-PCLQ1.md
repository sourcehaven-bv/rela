---
id: RR-PCLQ1
type: review-response
title: Verifier relocation has no concrete API in plan
finding: 'Plan says verifier moves out of fsstore into ''factory or storage/integrity'' — pick one. Verifier currently uses s.entitiesDir, s.relationsDir, s.attachDir and s.fs. After relocation, factory must hand it: raw storage.FS, all three paths, and mode enum. Plan stipulates the enum but not the rest. Chicken-and-egg: verification must run BEFORE New returns a usable store, so it cannot live behind the store API.'
severity: significant
resolution: 'Plan now specifies a concrete API: integrity.Verify(fs storage.FS, wantSealed bool, dirs []string) error in a new internal/storage/integrity package. Called by the factory between FS-stack assembly and fsstore.New, with the three dir paths passed explicitly. No more ''factory or storage/integrity'' hand-wave.'
status: addressed
---
