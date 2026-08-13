---
id: RR-27WGOX
type: review-response
title: A corrupt gitignored alias cache file bricked every rela command
finding: 'buildStateAndAliases treated caldavalias.ErrCorrupt as fatal, and it runs on EVERY appbuild path - rela list, analyze, the MCP server, the desktop app - almost none of which serve CalDAV. So a truncated .rela/caldav/aliases.json, a gitignored cache file, killed every command on a project with no caldav: block at all, citing a subsystem the user had never enabled and offering no remediation. Reproduced: `rela list task` on a bare project exited with ''build caldav alias service: caldavalias: stored aliases are corrupt''.'
severity: critical
resolution: 'Degrade instead: log a warning naming the file path and the remedy, and return a nil alias service. The fail-loud reasoning is still correct where it applies - serving CalDAV from an empty table makes every synced client re-create its entries as new entities - and registerCalDAVRoutes already refuses to mount without a healthy table, so that guarantee now lands on the path that can cause the damage. Verified live: rela list works with a warning; the server starts, REST works, and CalDAV returns 404 (not mounted). Test TestCorruptAliasTableDoesNotBrickTheBinary asserts both halves.'
status: addressed
---
