---
id: TKT-WRKFQL
type: ticket
title: Skip scheduled mail when no section has content visible to the recipient
kind: enhancement
priority: medium
effort: s
status: done
---

Opt-in `require_visible_content`: suppress a declarative scheduled mail entirely
when recipient-scoped ACL filtering (or the section `where:` clauses) leaves
every section empty, instead of delivering a message whose sections all read
"Nothing to show."

## Problem

Declarative scheduled mail fans out one child job per selected recipient
(`internal/scheduler/foreach.go`) and renders each message through the
recipient-scoped visible reader (`internal/appbuild/scheduled_mail.go:60` —
`mailtemplate.Build(ctx, s.meta, deps.VisibleReader, tmpl, ...)`).

When ACL row-gating removes every entity a template's sections matched, the
message is still sent. The recipient receives a message whose every section
renders the empty-state placeholder: `internal/mailrender/template.go:225` sets
`out.Empty = "Nothing to show."` for a section declaring columns with no rows,
and `internal/mailrender/text.go:82` writes the same string into the plain-text
alternative.

The result is recurring, content-free mail. It trains recipients to ignore the
channel, which is the failure mode that matters — a digest nobody opens is worse
than no digest.

## Desired behaviour

Opt-in per mail template:

```yaml
mail_templates:
  mt_agenda:
    subject: "MT-agenda {{today}}"
    intro: "De automatisch samengestelde agenda voor het eerstvolgende MT."
    address_property: email
    require_visible_content: true
    sections:
      - entity_type: mt_overleg
        where:
          - "datum = today"
          - "status = agenda_gereed"
        style: detail
```

When enabled, send only if at least one section has content visible to that
recipient. Default (absent/false) preserves today's behaviour exactly.

## Why this shape, and what it replaces

This ticket originated as a request for **relational filters in
`for_each.where`** — selecting recipients by a graph hop, e.g. every `persoon`
linked via `heeft_rol` to a `rol` titled "Managing Director". That framing was
discussed and rejected in favour of this one. The reasoning is worth keeping,
because the rejected options will look attractive again later:

- **Relation traversal in `for_each` duplicates the routing fact.** "Who is an
MD" would live both in `acl.yaml` (authorization) and `schedules.yaml` (mail
routing), and the two drift silently: an ex-MD keeps receiving mail until
someone remembers the second file.
- **`predicate` buys no query-plan win here.** `predicate` already has
`has_relation` / `count_relations`, and the compiler already parses the
named-args form `has_relation(entity, 'x', {k='v'})`
(`internal/predicate/walk.go:358`). But `SQLPortable` is *classification, not
lowering* (`internal/predicate/doc.go:112`), the pushdown path
(`queryplan.PushdownPrefilters`) consumes `[]*filter.Filter` rather than a
`*predicate.Program`, and `program.go:63` marks any `tableArgNode` non-portable
unconditionally. Selection stays an in-process scan either way.
- **ACL `role_relations` cannot express it.** `RoleRelationDef.Confers`
(`internal/acl/policy.go:670`) is a fixed role name keyed on relation *type*,
while `heeft_rol → rol` discriminates on the target entity's `titel`. Worse,
role-relation roles are attributed **per target entity** in `computeForEntity`
(`internal/acl/resolver.go:140`); `computeGlobals` (`resolver.go:18`) never
consults `RoleRelations`, so there is no "global role conferred by a relation"
to select on.

Deriving the audience from the content query dissolves all three: the MT agenda
goes to whoever has an MT agenda to read. Nothing restates the routing fact, and
revoking read access stops the mail with no `schedules.yaml` edit.

## Design notes

**Placement is on the template, not `for_each`.** The emptiness fact is computed
by `mailtemplate.Build` and acted on by `RunScheduledTemplate`; `for_each` is
generic fan-out that also serves `script:` tasks with no template at all, where
the key would be silently inert. Putting it on `Template`
(`internal/mailtemplate/mailtemplate.go:28`) keeps it inert by construction
where it does not apply.

**Emptiness = zero entities that CONTRIBUTED CONTENT** (revised by RR-K7RMIC;
the first draft said "matched", which was wrong). `mailtemplate.Build` already
tracks matches as `count` for `{{count}}` interpolation, but a match is not the
same as content: a `style: detail` section whose matched entity has empty
`Content` renders nothing, and counting it as present would send exactly the
empty message this ticket exists to prevent.

So `Build` gains a SECOND counter — contributions — decided in the same loop:
`table`/default and `list` always contribute, `detail` contributes only when
`strings.TrimSpace(ent.Content) != ""`. The existing `count` keeps its "entities
matched" meaning so `{{count}}` is unchanged.

Counting in the builder (rather than inspecting the rendered message) is
deliberate: emptiness lives in different fields per style — `detail`/`list`
accumulate `Section.Body`, `table` accumulates `Section.Rows` — so a predicate
over the rendered output would need updating for every future style.

Deliberately, an opted-out template whose sections matched nothing still sends
(it may carry a meaningful `intro`). Only opting in changes that.

## Scope

In scope:

- `require_visible_content` on `mailtemplate.Template`, parsed and validated.
- Suppression in `Services.RunScheduledTemplate` before `sender.Send`.
- An operator-facing log line when a send is suppressed.
- Tests per the acceptance criteria.

Not in scope:

- Relational filters in `for_each.where` (rejected above).
- Any change to `predicate`, `acl`, or the visibility wrappers.
- Automation-triggered mail (TKT-LU4AAY).

## Acceptance criteria

1. Recipient-scoped ACL filtering is unchanged — the same visible reader
renders content, and the raw recipient record is still used only to address the
envelope.
2. No mail is sent when every section is empty after ACL filtering and
`require_visible_content: true`.
3. Existing configurations retain current behaviour by default; a template
without the key, or with it false, sends exactly as today.
4. A suppressed send emits a diagnostic log line (Info) naming template and
recipient and **nothing else** — never entity titles or property values. This is
a diagnostic for an operator who raises the log level, not a default-visible
mitigation.
5. Tests cover fully hidden, partially visible, and fully visible content, and
the opted-out default.

## Test plan

| Scenario | Setup | Expected |
| --- | --- | --- |
| Fully hidden | Entities match `where:`, recipient may see none | No send; suppression logged |
| Partially visible | Recipient sees a strict subset | Send; only visible rows present |
| Fully visible | Recipient sees all | Send; unchanged from today |
| Opted out, empty | `require_visible_content` absent, nothing visible | Send, with "Nothing to show." — proves AC 3 |
| Multi-section, one non-empty | Section A empty, section B has rows | Send |

Existing ACL fixtures from TKT-MESVDG cover the recipient-scoped reader setup
and should be reused rather than rebuilt.

## Risks

- **Silent non-delivery is harder to diagnose. Accepted, not mitigated.** "Why
didn't I get the mail?" now has two indistinguishable causes: no matching data,
or no *visible* matching data. The AC 4 log line helps an operator who raises
the log level, but suppression is configured, non-actionable behaviour and is
logged at Info accordingly — it is not visible by default, and escalating it to
Warn to claim otherwise would warn about the feature working correctly.
- **Whether this suffices for the originating use case depends on the
deployment's ACL.** If `mt_overleg` entities are readable org-wide, every
recipient sees content and nobody is suppressed. This ticket is only a routing
mechanism where visibility is already scoped; it does not itself restrict
anything. Confirm before assuming it replaces role-based routing.
- **Cost is unchanged but not improved.** Selection still fans out to every
entity matching `for_each`, so a large recipient set still renders a message per
recipient to discover most are empty. `ForEachConfig.EffectiveLimit()` bounds
the children; this ticket does not change that.
