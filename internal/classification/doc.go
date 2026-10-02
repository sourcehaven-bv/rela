// Package classification reads and analyses a project's optional
// classification.yaml: operator-defined labels that describe what kind of
// data each entity and relation field holds (TKT-8UCV32, RES-TZH38L).
//
// # Metadata, not enforcement
//
// Classification describes data. It never blocks, denies or redacts
// anything, and it is not access control: acl.yaml stays the only place that
// decides access. CLI tooling reads classification to report where labeled
// data sits, to check that every field was reviewed, and to tell people
// writing ACL policy what a grant exposes. Whether a flow of labeled data is
// acceptable depends on context only the operator knows.
//
// # Outside core
//
// No runtime package (metamodel, store, API, write path) imports this
// package; arch-lint allows only the CLI to. A project without the file pays
// nothing. That is also why the package imports no other rela package: the
// CLI describes the schema to it as a [Shape] value.
//
// # One concept: labels
//
// A label is a name the operator chooses for a kind of data, at whatever
// granularity suits them ("health", or "special-category", or both on one
// field). There are no sensitivity levels, categories or framework bundles
// next to labels. A label may carry an identifier [Role], which says how that
// kind of data identifies a person; combination rules and subject inference
// compute with roles. A label with a `when:` rule is a derived label: it
// applies to a set of fields that together match the rule.
package classification
