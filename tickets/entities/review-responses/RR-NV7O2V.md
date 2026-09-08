---
id: RR-NV7O2V
type: review-response
title: 'Negative test asserts YAML behaviour that yaml.v3 does not have: require_visible_content: "yes" decodes to true, not an error'
finding: 'The plan''s Negative Tests section asserts that require_visible_content: "yes" (string, not bool) makes Parse return an error. Verified empirically against gopkg.in/yaml.v3 with a struct mirroring the proposed Template: "yes" decodes to true with NO error, bare yes decodes to true, and only quoted "true" errors (''cannot unmarshal !!str into bool''). yaml.v3 resolves YAML 1.1 booleans, so the plan''s prediction is inverted. The test as planned would fail immediately. Replace with the real behaviour: pin that quoted "true" errors (the spelling operators most likely reach for, and a counter-intuitive failure), that yes/"yes" are accepted as true, and keep the KnownFields(true) unknown-key rejection which was correct.'
severity: significant
resolution: 'Plan''s Negative Tests section rewritten to match empirically verified yaml.v3 behaviour: quoted "true" returns a parse error (pinned, since quoting is the spelling operators reach for and the failure is loud), while bare yes and quoted "yes" are both accepted as true per YAML 1.1 boolean resolution (pinned so a yaml library bump that changes it is caught). The KnownFields(true) unknown-key rejection was correct and is retained.'
status: addressed
---

## Finding

The plan's **Negative Tests** section asserts:

> `require_visible_content: "yes"` (string, not bool) → `Parse` returns an
> error, not a silent default.

This is false, and I verified it empirically against `gopkg.in/yaml.v3` with a
struct mirroring the proposed `Template` (bool field, `KnownFields(true)`):

| Input | Result |
| --- | --- |
| `require_visible_content: true` | no error, value `true` |
| `require_visible_content: "yes"` | **no error, value `true`** |
| `require_visible_content: yes` | no error, value `true` |
| `require_visible_content: "true"` | error — cannot unmarshal `!!str` into bool |
| `bogus_key: 1` | error — field not found |

yaml.v3 resolves the *unquoted* YAML 1.1 booleans (`yes`/`no`/`on`/`off`) to
bool, and a **quoted** `"yes"` also decodes into a bool field. Only `"true"`
fails — the opposite of what the plan predicts, and a genuinely surprising
asymmetry.

Writing the test as planned would produce a test that fails immediately, and
"fixing" it by asserting the real behaviour without understanding it would
silently enshrine a footgun.

## Why it matters

This is the plan's only negative test for the new key. As written it tests
nothing real, and the *actual* risk it should cover is inverted: an operator who
writes `require_visible_content: "true"` (quoting, as people habitually do in
YAML) gets a **hard parse error**, while `"yes"` silently works. That is worth a
test precisely because it is counter-intuitive.

## Resolution

Replace the negative test with what the decoder actually does:

- `"true"` (quoted) → `Parse` returns an error. Pin this, since it is the
spelling an operator is most likely to reach for and the failure is loud.
- `yes` / `"yes"` → accepted as `true`. Pin as documented YAML 1.1 behaviour so
a future yaml library bump that changes it is caught.
- Unknown neighbouring key still rejected by `KnownFields(true)` — keep, this
one was correct.

No production-code change; the defect is in the plan's assertion.
