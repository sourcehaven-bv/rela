---
id: RR-S86XJL
type: review-response
title: Pre-scope migration files strand content edges
finding: A to-projection without RelationScopes yields no content types, so content edges would stay on the zero tail with no row behind them.
severity: significant
resolution: relationScopes records whether scopes are known. A move of a row with edges under unknown scopes fails with errScopesUnknown and asks to regenerate the file. Validate cannot refuse because applied files are re-validated. Pinned by TestMigrateFace_RefusesEdgesWhenScopesAreUnknown.
status: addressed
---
