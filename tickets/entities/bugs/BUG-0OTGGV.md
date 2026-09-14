---
id: BUG-0OTGGV
type: bug
title: rela init generates a schema.yaml that fails to load, making new projects unusable
description: '`rela init` generated a schema.yaml whose entity definitions carried no `id_type`, which is exactly what the `short-id-default` migration detects. Detection is a hard gate rather than a warning: `metamodel.FSLoader.Load` returns a `*migration.Error` instead of loading, so a brand-new project would not open at all. `rela list` and every other command failed with "uses deprecated syntax" on a file rela itself had just written. Worse, the only path forward the error offered (`rela migrate`) writes `id_type: sequential`, so following the instructions silently pinned every new project to legacy sequential IDs — the opposite of the intended `short` default. Verified against the pre-fix binary built from develop.'
priority: high
effort: s
why1: The four entity definitions in DefaultMetamodelYAML() carried id_prefix but no id_type, and ShortIDDefaultMigration.Detect flags any entity missing id_type. Because metamodel.FSLoader.Load turns any detection into a hard error, that omission made the generated project unloadable rather than merely untidy.
why2: When the default id_type changed from sequential to short (FEAT-012), the change shipped a migration to move existing projects forward, but the init template was not updated alongside it.
why3: The template is a hand-maintained YAML string literal in loader.go that duplicates what DefaultMetamodel() expresses as a Go struct. The struct relied on the implicit default and so needed no edit, which made the YAML twin easy to overlook.
why4: 'Nothing tied the two artifacts together: no test compared the template against the migration set, so a template that had fallen behind the current syntax was indistinguishable from a correct one on every CI signal.'
why5: Migrations were treated as a tool for pre-existing user projects only. Generated output was never considered a consumer of the same syntax rules, so 'the migration set defines what current syntax is' was never asserted anywhere the generator could be checked against.
prevention: AM-init-schema-needs-no-migration asserts that the real Initialize output triggers no migration, checked against the WHOLE registered migration set rather than the one migration that caught this. Any future migration whose rollout forgets the init template now fails CI at the point of introduction, which addresses the why4/why5 systemic gap rather than only the id_type instance.
status: done
---
