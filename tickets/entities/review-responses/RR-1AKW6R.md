---
id: RR-1AKW6R
type: review-response
title: Guard covers only rela-server in rela_*; rela-server-postgres, rela-docs and rela are unguarded
finding: 'The packaged-binary step globs only `dist/rela_*_linux_amd64.tar.gz` and extracts only `rela-server`. Verified: `rela-postgres_1.2.3_linux_amd64.tar.gz` does NOT match the `rela_*` glob, so the postgres archive is never inspected. `go list -deps` confirms `internal/dataentry` is linked into cmd/rela-docs and cmd/rela as well as cmd/rela-server, so all of them embed the SPA. rela-server-postgres is a server whose whole purpose is serving that UI, in a separate archive, with zero coverage — the same bug class one archive over. The tree check running before GoReleaser makes drift unlikely, but the artifact-level check exists precisely to not rely on that reasoning.'
severity: critical
resolution: 'Packaged-binary step now loops over all SPA-embedding binaries across BOTH archives: rela-server + rela-docs from rela_*, and rela-server-postgres from rela-postgres_*. Verified empirically which binaries actually embed: rela-server, rela-docs and rela-server-postgres all match 37/37 entry assets. Notably the `rela` CLI does NOT embed the SPA despite linking internal/dataentry (the linker drops the unused embed), so asserting on it would have broken every release — it is deliberately excluded. Simulated the real step body against staged archives: passes with good binaries, exits rc=1 when the genuine broken v0.14 rela-server is swapped in. Single linux/amd64 archive per pair retained, with a comment stating why (all platforms share one embed source in the same job).'
status: addressed
---
