---
id: BUG-0OTGGV
type: bug
title: rela init writes a schema.yaml that rela migrate immediately flags as deprecated
description: '`rela init` generated a schema.yaml whose entity definitions carried no `id_type`. The `short-id-default` migration detects exactly that condition, so the very first `rela migrate status` in a brand-new project reported deprecated syntax and told the user to run `rela migrate` on a file they had just been handed. The generated schema was internally valid (`rela validate` passed) but was not in the current syntax.'
priority: medium
effort: s
why1: The four entity definitions in DefaultMetamodelYAML() carried id_prefix but no id_type, and ShortIDDefaultMigration.Detect flags any entity missing id_type.
why2: When the default id_type changed from sequential to short (FEAT-012), the change shipped a migration to move existing projects forward, but the init template was not updated alongside it.
why3: The template is a hand-maintained YAML string literal in loader.go that duplicates what DefaultMetamodel() expresses as a Go struct. The struct relied on the implicit default and so needed no edit, which made the YAML twin easy to overlook.
why4: 'Nothing tied the two artifacts together: no test compared the template against the migration set, so a template that had fallen behind the current syntax was indistinguishable from a correct one on every CI signal.'
why5: Migrations were treated as a tool for pre-existing user projects only. Generated output was never considered a consumer of the same syntax rules, so 'the migration set defines what current syntax is' was never asserted anywhere the generator could be checked against.
prevention: AM-init-schema-needs-no-migration asserts that the real Initialize output triggers no migration, checked against the WHOLE registered migration set rather than the one migration that caught this. Any future migration whose rollout forgets the init template now fails CI at the point of introduction, which addresses the why4/why5 systemic gap rather than only the id_type instance.
status: review
---
