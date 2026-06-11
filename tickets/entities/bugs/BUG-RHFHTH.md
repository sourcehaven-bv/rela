---
id: BUG-RHFHTH
type: bug
title: GenerateShortID can emit IDs its own validator rejects (pathological prefixes)
description: Fuzzing found GenerateShortID(prefix="--") returns "--9HHF", which entity.ValidateID rejects (consecutive dashes). The generator treats the prefix as opaque, so invalid prefixes flow into emitted IDs.
priority: low
effort: xs
status: ready
---

## Found by

The weekly fuzz sweep work (TKT-PCLGGL): 2 seconds of fuzzing
`FuzzGenerateShortID` — a target the per-PR fuzz job never runs — produced a
failing input.

## Reproduction

```text
go test fuzz v1
string("--")
string("0")
int(10)
string("0")
```

`GenerateShortID(existingIDs=["0"], prefix="--", entityCount=10, caps="0")`
returns `"--9HHF"`, which `entity.ValidateID` rejects: `consecutive dashes not
allowed in entity ID: --9HHF`.

(The failing input is deliberately NOT committed to
`internal/entity/testdata/fuzz/` — as a seed it would fail every `go test` run
until this is fixed. Re-find it with `go test -run='^$'
-fuzz='^FuzzGenerateShortID$' -fuzztime=30s ./internal/entity/`, or paste the
corpus block above into a file under `testdata/fuzz/FuzzGenerateShortID/` while
fixing.)

## Analysis (starting point)

`GenerateShortID` (internal/entity/id.go:219) treats the prefix as opaque: it
trims a trailing `-` and appends `-<base36>`. A prefix that itself violates ID
rules (`--`, control chars, path separators) flows straight into the result, so
the generator can produce IDs that `entity.ValidateID` — and the stores via
`storeutil.ValidateID` — reject. In practice prefixes come from the metamodel
(`id_prefix`), so hitting this requires a broken metamodel; check whether
`metamodel.Parse` already rejects malformed prefixes.

## Suggested direction

Either make `GenerateShortID` defensive (validate/sanitize the prefix; invalid
prefix is a programming/config error), or enforce prefix validity at metamodel
load and narrow the fuzz harness to metamodel-legal prefixes. Decide based on
what `metamodel` already guarantees.
