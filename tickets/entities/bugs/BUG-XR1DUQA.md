---
id: BUG-XR1DUQA
type: bug
title: The Changes list shows customers internal records and raw names
description: The Changes list of an entity page shows managed relations and properties, updates of hidden properties and raw audit names. It ignores header_buttons.history.
priority: high
effort: m
why1: 'The Changes list served every audit record of the entity that named only readable entities: managed _ relations, _ property updates and updates of hidden properties (as a bare ''updated''). The panel showed the raw op, principal, relation type, ids and summary.'
why2: The list was built for operators reading their own audit trail. Its gate asked only whether a record names an unreadable entity, not whether the reader should see the record at all.
why3: The field gate judged each named property but kept a bare 'updated' when it named none. Nothing judged rela's own _ names or relations.
why4: The panel reused describeSubject from the operators' audit view, which prints raw values, and was gated only on whether the build has an audit log.
why5: Reader-facing surfaces built on an operator data source inherit its raw names and its idea of what is worth showing, with no checklist item for customer-facing readers.
prevention: Tests pin that managed and hidden-only records are left out and that the panel shows labels and follows header_buttons.history.
started: "2026-11-19"
completed: "2026-11-19"
status: done
---

## Description

On a customer portal the Changes list of an entity page showed customers engine
internals:

- records of relations rela manages itself, such as a flow run's `_carries`;
- updates of `_` properties such as `_order`;
- a bare "updated" record each time a property the customer may not see
changed, which tells when a hidden value changed;
- raw audit names: ops (`update-entity`), principals (`system:scheduler`),
relation type names, property names, ids and trigger labels.

The list also ignored `header_buttons.history`, so a project that keeps the
history from customers still showed them the Changes list.

## Acceptance Criteria

- [ ] A record of a `_` relation type is left out of the Changes list.
- [ ] An update whose changed properties are all hidden from the reader, or
all `_` properties, is left out, and does not shorten a page.
- [ ] The list names relations, properties and types by label, other
entities by title, system principals as "System", and triggers by kind.
- [ ] Where `header_buttons.history` hides History, the Changes list is
hidden too.
- [ ] The behaviour is documented.
