---
id: BUG-C9ZYPD
type: bug
title: FuzzParse fails a user's own key back at itself when the key looks like a YAML tag
description: 'internal/metamodel FuzzParse asserts that Parse''s errors never leak Go/YAML internals, scanning the whole error string for patterns like `!!\w+`. The scan cannot tell a leaked tag from a quoted echo of the user''s input, so input "0!!0:" fails the target: Parse correctly answers `unknown key "0!!0" (valid keys: ...)` and the oracle reads the user''s own key as a `!!` tag leak. The production error is right -- naming the offending key is what makes it actionable -- so the defect is in the test oracle. Found by the weekly fuzz sweep (issue #993), reproducible in ~2s.'
priority: low
why1: The oracle regex-scanned the entire error message, including the %q-quoted span where the loader echoes the user's offending key back to them.
why2: The forbidden-pattern list was written to describe a leak by its BYTES (`!!str`, `metamodel.X`, `cannot unmarshal`), on the assumption that those byte sequences only ever originate from yaml.v3 or Go's reflect output.
why3: 'That assumption holds for the loader''s own prose but not for the message as a whole: an error that quotes user input necessarily contains arbitrary attacker-chosen bytes, so any byte-level blocklist over the full string is satisfiable by the input itself.'
why4: The property being tested was never stated in terms of provenance. 'The error must not contain !!str' and 'the loader must not emit an untranslated yaml tag' look identical on the loader's own outputs and diverge only on inputs that quote back. The fuzzer's job is to find exactly that divergence.
why5: A test oracle over a string that concatenates trusted prose with untrusted input needs to know which span is which; a blocklist that ignores provenance is a false-positive generator whose rate is set by the adversary. The fix is to elide the quoted spans before scanning, so the assertion is made about what the loader wrote rather than about what it was handed.
prevention: forbiddenPatterns is now scanned only via assertNoInternalsLeak, which routes through stripQuotedEchoes; a doc comment on forbiddenPatterns says never to match against the raw message. TestStripQuotedEchoes pins both directions -- a real leak in prose is still flagged, a quoted echo is not -- so the fix cannot decay into blanket-silencing the check.
status: done
---

## Description

The weekly fuzz sweep
([#993](https://github.com/sourcehaven-bv/rela/issues/993)) found
`./internal/metamodel FuzzParse [fuzz-crash]` on 2026-09-07 and again on a local
re-run of the sweep against current `develop`.

Minimized input:

```
go test fuzz v1
string("0!!0:")
```

Failure:

```
loader_fuzz_test.go:400: error contains Go/YAML internal "!!\\w+":
  unknown key "0!!0" (valid keys: attachments, automations, comments, copies,
  description, entities, includes, namespace, relations, transforms, types,
  validations, version, worlds)
```

## Why the production code is correct

`checkUnknownKeys` (`internal/metamodel/loader.go:1238`) formats the offending
key with `%q`. Naming the key is the entire value of the message: without it the
operator is told only that *some* top-level key is wrong. The `!!0` in the
output is the user's own text, echoed back inside quotes -- not a yaml tag that
escaped the humanizer.

## What the oracle actually needed to assert

`forbiddenPatterns` describes a leak by its bytes. That works for the loader's
own prose and fails for the message as a whole, because a message that quotes
user input contains arbitrary caller-chosen bytes. Four `!!`-shaped seeds were
already in the corpus from earlier sweeps; those were fixed by humanizing the
yaml error so no tag reaches the prose at all. This one is the residue of that
approach: there is no prose to fix.

## Fix

`stripQuotedEchoes` blanks out `%q`-quoted spans (length-preserving, so any
offsets printed on failure stay meaningful) and `assertNoInternalsLeak` scans
the result. Both `FuzzParse` and `FuzzParseErrorQuality` route through it.

`TestStripQuotedEchoes` pins the discrimination in both directions, so the
change is distinguishable from disabling the check.

## Verification

- The minimized input is committed as a seed and passes.
- `FuzzParse` fuzzed 90s clean (previously failed in 2.4s).
- `golangci-lint run internal/metamodel/...`: 0 issues.
