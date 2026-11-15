// Package mapimport imports external structured data into entities and
// relations through an operator-authored mapping file, imports/<name>.yaml
// (TKT-K7HR49Y). `rela import --mapping <name> <file>` runs it.
//
// # Pipeline
//
//  1. [Parse] reads and validates the mapping against the metamodel. Every
//     problem is a load error, so a typo fails before any data is read.
//  2. [Decode] turns the input (JSON, YAML, XML or CSV) into one [Node] tree
//     under size and node caps. XML goes through safexml, so no DTD is
//     processed and no external entity is resolved.
//  3. [Build] selects records with the mapping's paths, derives the entity
//     each record describes, runs the optional Lua hook, and compares the
//     result with the stored rows. The outcome is a [Plan]: creates,
//     per-property updates, relation adds and removes, rows that left the
//     source, and per-record errors. A dry run stops here.
//  4. [Apply] executes the plan through the entity manager.
//
// # Reads and the ACL
//
// The plan is a diff against the STORED rows, read raw like a write-prep
// read, because a redacted read would show a hidden property as absent and
// plan a write for it on every run. Every stored row the plan names is first
// read through the principal's gated reader ([Deps.Gate]); a row the
// principal cannot read is never named. A source key held by such a row is a
// record error, which tells the principal only that the key is taken. That is
// the same one-bit channel a unique constraint already has.
//
// Values come from the gated read too. A record that would compare or write
// a property the gated row hides (`visible:`) is a record error, so the plan
// never shows a hidden value or whether the source equals it. A row that
// hides its key or a scope property counts as unreadable, because matching
// it would tell which value it holds. Writes go through the manager, which
// authorizes, validates, runs automations and audits.
//
// # Identity
//
// A mapping identifies an entity by its key property, which must be
// `unique:` or an `external_ref`, optionally narrowed by `scope:` constants,
// or by an id template. Running the same import twice therefore plans no
// changes. Rows that left the source are reported, never deleted.
//
// # Hook
//
// The hook is a pure transform in a bare Lua sandbox (lua.NewSandbox): no
// rela bindings, no io, os, http, AI or mail. It receives the derived draft
// and the source record and returns the draft or nil to skip the record. It
// cannot write, so a dry run shows exactly what an apply would do.
package mapimport
