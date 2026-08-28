---
id: RR-VXZEUN
type: review-response
title: TestDomainRedactedNotOnWire was vacuous — asserted against its own helper, structurally unfalsifiable
finding: 'The test I added for RR-79L852 built its own v1.Entity via a toWireForTest helper that hardcoded {ID, Type, Properties}. Since v1.Entity DOES have a Redacted *[]string field (apiwire/v1/responses.go:61), the helper structurally could not propagate the domain field regardless of what production code does. The comment claimed it would catch ''a future struct-copying serializer'', which it cannot — such a serializer would not call the helper. Worse than no test: it advertised coverage that did not exist, and it is exactly where the command-stdin leak (RR-JHQ2CX) would have surfaced had it driven real code.'
severity: significant
resolution: 'Rewritten to drive the real entitySerializer.toV1 with a sentinel value (''DOMAIN-SIDE-LEAK'') in the domain Redacted field, asserting it never appears in the marshalled wire output. Verified FALSIFIABLE by mutation testing: temporarily propagating e.Redacted into out.Redacted inside entityserializer.go made the test fail with the sentinel visible in _redacted; reverting restored PASS. The helper was deleted.'
status: addressed
---

## Finding (from cranky-code-reviewer)

`internal/dataentry/affordances_test.go` — the original test:

```go
blob, _ := json.Marshal(toWireForTest(e))
if strings.Contains(string(blob), "salary") { ... }

func toWireForTest(e *entity.Entity) v1.Entity {
    out := v1.Entity{ID: e.ID, Type: e.Type, Properties: map[string]any{}}
    maps.Copy(out.Properties, e.Properties)
    return out
}
```

The helper never sets `Redacted`, so the assertion could not fail no matter what
production did. The reviewer proved it by stuffing `"TOTALLY-SECRET"` into the
domain field and watching the test pass.

This is a self-inflicted version of the exact class of problem the finding it
was written for (RR-79L852) warned about — I wrote a test that mirrored the
shape I *believed* production had, instead of calling production.

## Resolution

Now calls `app.serializer.toV1(...)` — the real serializer, via the existing
`newTestAppV1` fixture (matching the precedent in `inaccessible_test.go`).

**Mutation-tested to prove falsifiability**, which is the part that was missing
the first time:

```
# with a temporary leak injected into entityserializer.go:
--- FAIL: TestDomainRedactedNotOnWire
    domain Redacted field reached the wire: {... "_redacted":["DOMAIN-SIDE-LEAK"]}
# reverted:
ok
```

## Lesson

A test asserting "X never appears in the output" is only meaningful if the
output is produced by the code under test. Building the expected shape by hand
guarantees the assertion holds — which reads as safety and is the opposite.
