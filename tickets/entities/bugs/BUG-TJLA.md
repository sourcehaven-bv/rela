---
id: BUG-TJLA
type: bug
title: 'data-entry config: misleading ''unknown relation'' error for inverse names and wrong-side form bindings'
description: 'On data-entry forms, using an inverse relation name (e.g. `relation: connects_from` for the inverse of `connects_to`) was rejected at config-load time with `references unknown relation`, with no hint that the supported shape is `relation: connects_to` + `direction: incoming`. Worse, the natural workaround — using the canonical name on a form whose `entity_type` is on the wrong side of the edge — passed validation but silently produced a broken widget: the picker searched the wrong target type and existing edges never rendered. Reported in GitHub issue #780.'
priority: medium
effort: xs
why1: '`validateForms` rejected any relation name that wasn''t a canonical metamodel relation, including declared inverse names'
why2: the validator only called `meta.GetRelationDef`, never `meta.InverseOwner`, so it couldn't distinguish an inverse name from a typo
why3: and it never checked that the form's `entity_type` sat on the side of the relation the chosen direction implied, so a wrong-side binding produced no error at all — only a silently broken widget at runtime
why4: the original validator pass treated relation references as a flat name lookup and didn't reason about edge endpoints; the runtime layers downstream (`resolveDirection`, the direction-aware widget) already handled inverse and direction correctly
why5: there was no test exercising the validator with a metamodel that declared `inverse:`, so neither failure mode had ever produced a clear error message
prevention: '`validateForms` now (a) resolves inverse names via `Metamodel.InverseOwner` and points users at `relation: <canonical>` + `direction: incoming`, and (b) checks the form entity type against `relDef.From`/`relDef.To` for the chosen direction, hinting at the direction flip when the entity is on the opposite side. Table-driven tests in `validate_test.go` pin every branch using a metamodel parsed through the real loader so `inverseOwners` is populated the same way the runtime sees it.'
status: done
---
