---
id: RR-W7HKMAM
type: review-response
title: Fail closed when an applied migration does not converge
finding: The plan applies detected migrations and parses the result but never checks that the migrated tree is clean. A migration whose Apply leaves its pattern in place would be reported as applied while the strict parser sees a partly migrated file.
severity: significant
resolution: MigrateBytes re-runs detection on the migrated tree; any non-advisory detection that remains refuses the load with *migration.Error.
status: addressed
---
