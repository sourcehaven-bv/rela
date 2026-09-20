---
id: RR-385HBF
type: review-response
title: auto_open still shipped in the API response, advertising a removed capability
finding: The launcher removal left auto_open inert but still populated into ResolvedCommand and v1.Command, so /api/v1/_commands kept sending it, and frontend/src/types/config.ts kept typing it. Keeping the YAML key for config compatibility is right; keeping it in the response advertises a capability the server no longer has.
severity: minor
resolution: Removed AutoOpen from v1.Command, ResolvedCommand, and the TS Command interface. Jeroen chose removal over documenting it as served-but-ignored, and asked that `rela migrate` clean up project configs — so added a CommandAutoOpenMigration that strips the key from every entry in `commands:`. The YAML field stays on dataentryconfig.CommandConfig so an unmigrated project still loads; the migration is tidy-up, not a precondition for starting. Verified end-to-end through the real CLI, including idempotence. The old propagation test was inverted to pin that the key no longer reaches the wire.
status: addressed
---
