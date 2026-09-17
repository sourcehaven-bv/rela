---
id: BUG-6LCF9L
type: bug
title: analyze validations drops the message a Lua rule returns
description: |-
    A Lua validation rule signals a violation by returning `{ message = ... }`. The message is per-ENTITY: it says which of the rule's several possible defects this entity has, and what to do about it.

    `runLuaForEntity` (internal/validation/validation.go) assigned that message to `Violation.Description`, the field holding the RULE's own text. Both CLI table renderers group violations by Description, so the script message became the rule heading and each row printed only `id: title` — for entities without a title, a bare `ID:` and an empty field. In `-o json` the message appeared but as Description, so the rule text it displaced was lost. Table showed the description and lost the message; JSON showed the message and lost the description.

    Same defect on two more surfaces. Data entry never saw the message at all: `validator.RuleViolation` carried only EntityID/Face/Detail, so `analyzeValidations` fell back to `rule.Description` for every row. MCP `analyze_validations` returned bare entity id strings.

    Found while adding two `procedure` rules to atlas (ISMS): two entities failed the same rule for different reasons and were indistinguishable in the output.
priority: medium
effort: s
why1: The operator sees an empty field after the entity ID instead of the message the rule returned, and two entities failing the same rule for different reasons look identical.
why2: runLuaForEntity wrote the script's message into Violation.Description, and the table renderers group by Description and print only EntityID/EntityTitle per row — so the message was consumed as the rule heading and never reached the row.
why3: Violation had no field for a per-entity explanation. Description was the only free-text slot, so the Lua path reused it rather than adding one, and the struct comment endorsed this ("description overrides rule.Description for Lua-sourced violations").
why4: The two facts have different scopes — the rule description is per-rule, the script message is per-violation — but they were modelled as one field, so every renderer had to pick one to lose. Which one it lost depended on whether it grouped (table lost the message) or serialized flat (JSON lost the description).
why5: 'Systemic: a value whose cardinality differs from its container was folded into an existing field instead of getting its own, and no test asserted the two coexist. The Lua tests only ever checked the script message, using Description as the place to find it, so they passed in the presence of the bug and encoded it as expected behaviour.'
prevention: 'P2: Violation.Message holds the per-entity text and Description stays the rule''s; carried through validator.RuleViolation, the data-entry wire (ruleMessage) and MCP. P4: TestLuaValidation_MessageAndDescriptionAreSeparate asserts both fields on one violation, and TestNonLuaViolation_HasNoMessage pins the empty case so renderers can rely on it. Both fail if either field is folded back into the other.'
status: backlog
---

A Lua rule's message answers "what is wrong with THIS entity"; the rule's
`description:` answers "which rule fired". They are different levels, so both
are reported and neither substitutes for the other.

Before:

```text
⚠ Elke procedure moet een terugkerende taak hebben (2):
  PROCEDURE-91XS:
  PROCEDURE-MCBL:
```

After:

```text
⚠ Elke procedure moet een terugkerende taak hebben (2):
  PROCEDURE-91XS: Toegangsbeheer — geen terugkerende taak gekoppeld
  PROCEDURE-MCBL: Incidentbeheer — alleen uitgeputte taken; plan een nieuwe
```

A rule that returns no message (every non-Lua rule) keeps the bare `id: title`
form: the rule description is then the whole finding, and a trailing separator
would suggest text that is not there.

`rela validate` additionally took each rule heading from an arbitrary
violation's Description, which while Lua messages lived there meant one
entity's message was shown as if it were the rule. Now well-defined.
