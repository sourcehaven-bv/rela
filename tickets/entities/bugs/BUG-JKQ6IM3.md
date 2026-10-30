---
id: BUG-JKQ6IM3
type: bug
title: rela migrate flattens folded block scalars in project YAML
description: A migration rewrite puts each folded (>) block scalar on one long line and adds a blank line after it.
priority: medium
effort: s
why1: The migration runner decodes the file into a yaml.Node and writes it back with the yaml.v3 encoder. The encoder keeps the folded style but puts the whole value on one line and adds a blank line after a clipped block.
why2: yaml.v3 sets its line width to unlimited and has no option to change it (v3.0.1 has no SetWidth). So it never folds a long scalar.
why3: The runner was written for small structural edits and its tests only compared values and keys. No test kept a long folded scalar or checked the layout of the rewritten file.
why4: Config files are treated as data for the migration code. Their hand-written layout was not seen as something a migration must keep.
why5: There is no shared rule or test helper that a YAML rewrite must keep the operator's formatting where the value does not change.
prevention: TestApply_KeepsFoldedScalarsFolded and TestRefoldFolded pin the layout. refoldFolded checks that the re-wrapped output decodes to the same value and falls back to the encoder output if not.
started: "2026-10-30"
completed: "2026-10-30"
status: done
---

## Description

`rela migrate` rewrites a project YAML file (schema.yaml, data-entry.yaml,
acl.yaml) when a migration applies. The rewrite turns every folded block scalar
(`>`) into one long line, and a clipped (`>`) or stripped (`>-`) block gains a
blank line after it. The value stays the same, but the file the operator wrote
by hand becomes hard to read and review.

Reported by the GRC demo agent: the `acl-scheduler-grant` migration rewrote the
demo's `acl.yaml` and its multi-line `description:` became one line of several
hundred characters.

## Reproduction

1. A project whose `acl.yaml` has a `description: >` paragraph over several
lines and no `scheduler-system` grant.
2. Run `rela migrate`.
3. The paragraph is now on one line, followed by an empty line.

`TestApply_KeepsFoldedScalarsFolded` in `internal/migration` fails on develop.

## Fix

After encoding, re-wrap folded block content lines at 80 columns, at single
spaces only, and drop the trailing blank lines that chomping discards. Check
that the result decodes to the same value; otherwise write the encoder output
unchanged.
