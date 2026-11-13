---
id: RR-7NGQ1ZM
type: review-response
title: AC6 coverage missed pending migrations, side effects and fixture comments
finding: The world-load test did not cover a pending generated migration, a side-effect automation, or comments in a fixture.
severity: significant
resolution: AC6 now generates a migration with datamigration.Generate, runs a write_file plus rela.piles.add automation, and loads fixture comments.
status: addressed
---
