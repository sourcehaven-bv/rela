---
id: RR-DOCDRIFT
type: review-response
title: autocascade's string mirror of the enum has no test pinning it to the metamodel constants
finding: 'scriptrunner.go:87-92 duplicates ACLBypassRead/Write/ReadWrite as plain strings to keep the
  package schema-free, with a comment saying they "MUST match the metamodel constants". Nothing checks
  it.


  A one-line test in a package that may import both (internal/script already does) pinning string(metamodel.ACLBypassRead)
  == autocascade.ACLBypassRead costs nothing and turns a comment into a guarantee. Today a typo in either
  place silently disables elevation on the cascade path -- fail-closed, but silently.'
resolution: |
  Added TestACLBypassConstantsMatchMetamodel in internal/script (the package that may import both), pinning autocascade.ACLBypassRead/Write/ReadWrite to the metamodel constants they duplicate.
  
  The comment records why it lives there and what it prevents: a typo in either place silently disables elevation on the cascade path — fail-closed, but silently, which is the worst shape for a capability gate.
severity: minor
status: addressed
---
