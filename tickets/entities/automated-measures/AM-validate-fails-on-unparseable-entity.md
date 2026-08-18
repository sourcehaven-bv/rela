---
id: AM-validate-fails-on-unparseable-entity
type: automated-measure
title: 'SUPERSEDED by AM-validate-fails-closed-on-unparseable-entity'
description: Guards the failure mode where a malformed entity is silently dropped from the result set and validate still exits 0. A fixture entity with unparseable frontmatter must make validate exit non-zero and name the offending path.
kind: ci
location: internal/cli/validate.go + tickets CI job in .github/workflows/ci.yml
status: deprecated
---

> **Superseded.** Filed alongside BUG-RMCK9U, which is a duplicate of
> BUG-NEQRY2; the surviving measure is
> `AM-validate-fails-closed-on-unparseable-entity`. Kept as `deprecated` rather
> than deleted so the `adds-measure` relation from BUG-RMCK9U stays intact and
> the trail is readable.
>
> Note the two describe slightly different guards, and the survivor is the
> better one: this measure named `ci.yml` as a location, which is what
> #1363 actually shipped — a grep for a log line. The survivor targets
> `internal/cli/`, i.e. the exit code itself, which is the guarantee that
> should not live in a pipeline file.
