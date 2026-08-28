# Ticket tracker — operator handbook

`rela description()`

This handbook is authored in Markdown; the tables, diagrams, and screenshot
below are resolved from the project's `schema.yaml` and `acl.yaml` by
`rela-docs build`, so they can never drift from the schema. It doubles as a
worked example of the rela-docs generator — see
[the guide](../rela-docs.md).

## Tickets

A **ticket** records a piece of work. Every ticket carries these fields:

```rela
typeref{ type = "ticket", fields = "required" }
```

### Status

A ticket moves through the lifecycle below.

```rela
lifecycle{ type = "ticket", field = "status" }
```

Each status means:

```rela
values{ type = "ticket", field = "status" }
```

### How a ticket connects to the rest of the tracker

```rela
relations{ type = "ticket" }
```

The schema neighbourhood, two hops out:

```rela
graph{ from = "ticket", depth = 2 }
```

## A worked example

```rela
local t = create("ticket", { title = "Login page 500s under load", status = "in-progress" })
entity{ id = t.id, fields = { "title", "status" } }
```

There is `rela count{ type = "ticket" }` seeded ticket in this example.

```rela
shows{ type = "ticket", exactly = { "ticket-1" } }
```

## Who can do what

The access model comes straight from `acl.yaml`:

```rela
roles_matrix{ type = "ticket" }
```

The table above is rendered from `acl.yaml`. These claims are _checked_ against
the real authorization path when this handbook builds, so the prose cannot
outlive the policy it describes:

```rela
-- An editor maintains the backlog.
permits{ who = "alice@example.com", op = "update", type = "ticket" }
permits{ who = "alice@example.com", op = "delete", type = "ticket" }

-- A viewer browses and nothing more. If someone widens `viewer` in acl.yaml,
-- this handbook stops building rather than quietly describing a policy that
-- is no longer in force.
refuses{ who = "bob@example.com", op = "update", type = "ticket" }
refuses{ who = "bob@example.com", op = "create", type = "ticket" }
refuses{ who = "bob@example.com", op = "delete", type = "ticket" }

-- There is no self-service sign-up: an unassigned principal gets nothing.
-- `unassigned = true` states that the missing assignment IS the claim, so this
-- cannot be confused with a typo in the principal name.
refuses{ who = "carol@example.com", op = "update", type = "ticket", unassigned = true }
```

## The edit form

This is the ticket edit form as an editor sees it, with the two lifecycle-driving
fields highlighted:

```rela
local demo = create("ticket", {
  id = "DEMO-TICKET", title = "Login page 500s under load",
  status = "in-progress", priority = "high", reporter = "demo@example.com",
})
screenshot{
  view = "form", type = "ticket", entity = demo.id,
  arrows = {
    { at = "status",   text = "the lifecycle state" },
    { at = "priority", text = "triage priority" },
  },
  clip = "focus",
  out = "ticket-form.png",
  alt = "The ticket edit form, with the status and priority fields highlighted",
}
```
