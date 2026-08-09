---
id: RR-Y15LRA
type: review-response
title: Migration step is unnecessary — unknown nested YAML keys are silently ignored
finding: 'The plan promises a rela migrate step dropping allow_create/create_form "so no config errors". There are no errors to prevent: checkUnknownKeys (validate.go:149-168) iterates top-level keys only; nested struct unmarshal is non-strict yaml.Unmarshal, so unknown nested keys are silently ignored. Repo-wide, zero form relations set either key (all 16 create_form: hits in tickets/data-entry.yaml are on lists/kanbans, which are kept). The justification is false and would mislead a reviewer.'
severity: significant
resolution: Migration removed from scope. Verified checkUnknownKeys validates top-level keys only and nested unmarshal is non-strict, so a stray key is a silent no-op; no form relation in the repo sets either key. Scope note now states the reason so it isn't re-derived.
status: addressed
---

## Resolution

Migration removed from scope. Verified: `checkUnknownKeys` only validates
top-level keys, nested unmarshal is non-strict, and no form relation in the repo
sets either key — so deleting the struct fields makes any stray key a silent
no-op, not an error.

Scope note updated to say the fields are deleted with **no migration needed**,
and to state the reason (non-strict nested unmarshal) so the next reader doesn't
re-derive it.
