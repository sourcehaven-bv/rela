# Ticket tracker — handleiding

`rela description()`

This manual is authored in Markdown; the tables and diagrams below are
resolved from the schema by `rela docs build`, so they cannot drift from
`metamodel.yaml`.

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

## Who can do what

```rela
roles_matrix{ type = "ticket" }
```

## The edit form

This is the ticket edit form as an editor sees it:

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
  out = "ticket-form.png",
  alt = "The ticket edit form, with the status and priority fields highlighted",
}
```
