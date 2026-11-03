---
id: RR-7IPHO7
type: review-response
title: Dead error path from crypto/rand.Read
finding: 'internal/storage/tempfile.go:45-48, safefs.go:128-133, attachment.go:80-83: go.mod is go 1.26, and since Go 1.24 crypto/rand.Read never returns an error (it crashes the program instead). TempPath''s error return, the err branch in createTemp and its coverage-ignore block (mislabelled os-fs-event), and the attachment nonce branch are dead, and the createTemp comment about a broken random source is wrong. Fix: make TempPath return string only, drop the branches and coverage-ignores, and reword the tempCreateAttempts comment (the bound now only guards against a real name clash).'
severity: minor
resolution: TempPath returns a plain string; the dead crypto/rand error branches and their coverage-ignores are gone, and the tempCreateAttempts comment is corrected.
status: addressed
---
