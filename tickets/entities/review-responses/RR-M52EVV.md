---
id: RR-M52EVV
type: review-response
title: 'Combined review: on the postgres build, screenshot{} would seed the live database'
finding: internal/cli/docs.go + docscapture carry no build tags, so they compile into rela-postgres. standUp calls appbuild.Discover, which on the postgres build binds pgstore to the shared RELA_DATABASE_URL — the 'throwaway temp project' only isolates the schema files on disk; entities go to the real schema-scoped DB. A screenshot-bearing manual on rela-postgres would write fixtures into (or fail against) live data. The ephemeral-project premise silently doesn't hold on that backend.
severity: significant
resolution: 'Build-tagged seam: newDocsCapturer() returns a real docscapture.Capturer on !postgres, and on the postgres build returns a nil capturer + a clear error, so screenshot{} fails loud (''not available on the postgres build (it would seed into the live database)''). Tier-A resolvers (tables/diagrams/matrices) still work on the postgres build. docscapture is no longer linked into the postgres binary.'
status: addressed
---
