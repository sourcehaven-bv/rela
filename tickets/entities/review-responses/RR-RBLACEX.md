---
id: RR-RBLACEX
type: review-response
title: rela test must bypass the CLI project bootstrap
finding: 'cli/kong.go:233-262 runs appbuild.Discover for requiresProject commands: opens live store, bleve lock, sqlite exclusive lock or pg DSN.'
severity: critical
resolution: 'test is excluded from requiresProject; it resolves the root with project.Discover and builds its own world. New AC: rela-sqlite test works while another process holds the db lock.'
status: addressed
---
