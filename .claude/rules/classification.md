---
paths:
  - "internal/classification/**"
  - "docs/classification.md"
---

# Data classification

- **Data classification describes; it never drives behavior** (TKT-8UCV32).
  `classification.yaml` labels what data each field holds, and
  `internal/classification` parses, lints and syncs it. Only `internal/cli`
  may import that package (arch-lint enforces it), and nothing reads it at
  runtime: no redaction, no access decision, no refusal keyed off a label.
  Behavior a label might suggest (searchable, logged, exported) is declared
  in core config. `rela acl audit` lists what each role can read of labeled
  data, and those findings never count toward `--fail-on`. Classification
  warns and never blocks, because whether labeled data may flow somewhere
  depends on context only the operator has. A field's two
  explicit states, `none` and `needs-review`, keep "not sensitive" apart from
  "not looked at"; do not add an implicit default. See
  `docs/classification.md`.
