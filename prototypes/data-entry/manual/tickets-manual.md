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
