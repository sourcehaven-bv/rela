---
id: RR-D00YBI
type: review-response
title: No test for landing {world} against a configured default_world
finding: 'EntityDetail.world.test.ts covers landing {world: published} with no defaultWorld set. The branch worldQueryFor exists for (drop the param when the target is the configured default) is untested.'
severity: minor
resolution: 'Two EntityDetail tests: landing in the configured default world drops the param (and the page), and landing on `default` under a configured default spells `?world=default`.'
status: addressed
---
