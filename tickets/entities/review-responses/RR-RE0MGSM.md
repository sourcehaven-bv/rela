---
id: RR-RE0MGSM
type: review-response
title: Chained migration refused with advice that needs two runs
finding: The post-apply check refused when one migration's output triggered another.
severity: minor
resolution: MigrateBytes repeats until nothing is pending, capped at one pass per registered migration. Pinned by TestMigrateBytes_FollowsChainedMigrations.
status: addressed
---
