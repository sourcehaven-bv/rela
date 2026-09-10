---
id: test-oracle-respects-input-provenance
type: automated-measure
title: A test oracle over a message that quotes untrusted input must scan by provenance, not by bytes
description: 'A blocklist over a string that concatenates trusted prose with untrusted input is satisfiable by the input. BUG-C9ZYPD is that case: FuzzParse forbade `!!\w+` anywhere in Parse''s error, and the input "0!!0:" put those bytes there itself, because the loader quotes the offending key back at the user -- which is the single thing that makes the error actionable. The fuzzer''s job is to find exactly this divergence, so the target failed on correct production behaviour. The measure: before asserting a forbidden byte sequence is absent from a message, elide the spans that echo caller-controlled input, and pin the discrimination with a unit test that a real leak in the prose is still caught.'
kind: test
location: internal/metamodel/loader_fuzz_test.go (forbiddenPatterns / assertNoInternalsLeak)
status: proposed
---

## The failure mode

Two properties look identical on the code's own outputs and diverge only on
inputs that get quoted back:

1. "the error must not contain `!!str`" — a byte-level blocklist
2. "the loader must not emit an untranslated YAML tag" — the property actually
wanted

A fuzzer searches precisely for the gap between them. Since the message embeds
the caller's bytes verbatim, the adversary sets the false-positive rate.

## The measure

Where a test asserts that a forbidden token is absent from a message built by
concatenating trusted prose with untrusted input:

- Elide the untrusted spans before scanning. In Go, `%q`-quoted runs are the
form the echo takes, so a length-preserving blank-out of quoted spans restores
the provenance the blocklist assumed.
- Pin both directions in a unit test. A one-directional test cannot tell the
fix from deleting the assertion, which is the cheap way out under fuzz-sweep
pressure.

## Where it applies beyond the finding

Any oracle over user-facing diagnostics: validator messages, CLI errors, API
error bodies. The same shape appears wherever a redaction or leak check runs
over a string that deliberately includes what the user supplied.

Sibling of [[emitted-bytes-assertion-at-scan-boundaries]] — both are cases where
the oracle's model of the string under test is wrong, not the code's.
