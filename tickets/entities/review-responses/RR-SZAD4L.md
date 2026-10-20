---
id: RR-SZAD4L
type: review-response
title: Reserved op names duplicated across packages
finding: 'realtimecfg/validate.go:34 hard-codes reservedActionNames {delete, edit, release, remove}, realtime/msgops.go:12-24 declares OpEdit/OpDelete/OpRemove/OpRelease, and frontend types/channels.ts MESSAGE_OPS lists them a third time. Add a fifth op and only one place gets it. realtimecfg is the lower package: define the op names there (realtimecfg.MessageOpNames) and have realtime''s consts and the validator use them. That also makes the ''reserved'' rule a single source. If realtime.yaml has shipped in a release, also note that an existing action named `release`/`edit` now fails to load and realtime goes off. Mention it in the changelog.'
severity: minor
resolution: Reserved message operation names now come from realtimecfg.MessageOpNames, realtime and validation use it, a Go golden writes them and the SPA test compares MESSAGE_OPS to that file, and the realtime guide has an upgrade note.
status: addressed
---
