// Package perfseed generates a deterministic, realistically shaped entity
// graph for the perf demo project (prototypes/perf/project) and loads it
// into any store.Store.
//
// # Why a generator and not a fixture
//
// The read paths this data exists to measure — list pages with relation
// columns, views with table sections, kanbans, the dashboard, the gantt —
// misbehave in proportion to graph size, and a checked-in fixture large
// enough to show that would be tens of megabytes of markdown. A generator
// with a seed reproduces the same graph on every machine from a few hundred
// lines, and `scale` turns the same shape up or down.
//
// # Determinism
//
// Every entity and relation is a pure function of (seed, kind, index): each
// gets its own PRNG seeded from those three values, so the entity stream and
// the relation stream can be produced independently and still agree on
// which policies have a published face, which documents have a Dutch one,
// and which project window a task falls inside. Nothing is drawn from a
// shared stream, so adding a property to one type cannot shift another
// type's values.
//
// # What it writes, and what it does not
//
// [Load] writes through store.Store directly — no entitymanager, so no
// automations, validations, or ACL. That is the fourth sanctioned raw-store
// path (after `db migrate`, `history-purge` and data migration) and carries
// the same obligations: operator-shell trust, store.WithAttribution, and an
// explicit audit record, all supplied by the `rela dev seed` command. The
// generator keeps itself honest where the store cannot: ids are minted by
// construction and validated, the one `unique:` property (person.email) is
// unique by construction, and every relation endpoint names an entity the
// same generator emits.
package perfseed
