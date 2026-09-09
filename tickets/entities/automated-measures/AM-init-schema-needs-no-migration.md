---
id: AM-init-schema-needs-no-migration
type: automated-measure
title: A freshly initialized project's schema.yaml triggers no migration
description: Runs the real projectsetup.Initialize against a MemFS and asserts migration.Detect reports nothing for the generated schema.yaml. Because it asserts against the whole registered migration set rather than one named migration, it also catches any FUTURE migration whose rollout forgets the init template — which is the recurring shape of this defect, not just the id_type instance.
kind: test
location: internal/projectsetup/init_migration_test.go
status: active
---
