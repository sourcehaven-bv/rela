---
id: RR-KMEOQT
type: review-response
title: Type set unknown for mentions collections
finding: collectionTypes returns nil for a rule without a relation, so *_by_type, sort and fallback column checks are skipped; a shared collect_as gives a wrong type set.
severity: significant
resolution: collectionTypes returns every entity type for a mentions collection; a shared collect_as is refused; the fallback list is validated against the uncovered types.
status: addressed
---
