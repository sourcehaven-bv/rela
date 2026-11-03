---
id: RR-M7BXFD
type: review-response
title: Loader plural check turns previously loading projects into hard load failures, on every backend
finding: 'internal/metamodel/loader.go:387 applies storage.ValidateDirSegment (ASCII [A-Za-z0-9][A-Za-z0-9_-]*) to def.GetPlural(name). ValidateSchemaName still accepts spaces, dots and non-ASCII letters in type names, so a type named "work item" or "categorieë" with no plural: now fails Parse because its defaulted plural fails, and so does any declared plural with a space (the basecamp case) or an accent. Parse failing means the project no longer starts at all, including on postgres and sqlite, where the plural names no directory. There is no internal/migration step and no release note. The message is also confusing in two ways: for a defaulted plural it reports "plural: ..." although the author never wrote one, and it does not say that the type''s existing entities/<old>/ directory must be renamed too, otherwise its entities silently disappear on the next open (unknown directories are skipped). Fix: keep the hard error for path-significant names only (''/'', ''\\'', ''.'', ''..'', leading ''.'', control characters) and make the rest a warning for one release, or add a migration. Word the error as: entity "x": the directory name "x s" is not allowed; set plural: to letters, digits, ''-'' or ''_'' and rename entities/x s/ to match. When the plural was defaulted, say "set plural: explicitly".'
severity: significant
resolution: 'The rule was loosened to the escape set only: spaces, accents, uppercase and inner dots load again, the examples/basecamp change is reverted, and the loader error names the type and says whether the plural was declared or derived.'
status: addressed
---
