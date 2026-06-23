---
id: BUG-RHFHTH
type: bug
title: GenerateShortID can emit IDs its own validator rejects (pathological prefixes)
description: Fuzzing found GenerateShortID(prefix="--") returns "--9HHF", which entity.ValidateID rejects (consecutive dashes). The generator treats the prefix as opaque, so invalid prefixes flow into emitted IDs.
priority: low
effort: xs
why1: 'GenerateShortID emitted "--9HHF" for prefix "--": it trims one trailing dash and appends "-<base36>" without checking what that produces.'
why2: The generator treats the prefix as opaque because prefixes were assumed to be sane — they only enter the system via the metamodel's id_prefix/id_prefixes.
why3: metamodel.Parse validated prefix declaration shape (conflicting forms, presence for short/sequential types) but never the character contract, so nothing between YAML and ID generation owned prefix validity.
why4: The ID character rules lived only in the consumers (entity.ValidateID, storeutil.ValidateID); there was no producer-side contract for inputs to generation, and no test generated IDs from hostile prefixes — the fuzz target's oracle hand-modeled per-character validity and missed dash runs.
why5: 'Systemic: contracts enforced only at the consuming edge let invalid values travel from config to the point of use before failing; the fuzz sweep (TKT-PCLGGL) exists precisely to surface such gaps, and this was its first catch.'
prevention: 'Load-time gate: metamodel.ValidateIDPrefix enforced in Parse (typed InvalidIDPrefixError), so a broken metamodel fails at startup, not at write time. The fuzz oracle now delegates to the same exported contract instead of hand-modeling it (the staleness class from TKT-PCLGGL), the repro is a committed regression seed, and the weekly fuzz sweep (adds-measure: weekly-fuzz-sweep) keeps generating hostile inputs against the gate.'
status: done
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
